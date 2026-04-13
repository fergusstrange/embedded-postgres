package embeddedpostgres

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var mu sync.Mutex

var (
	ErrServerNotStarted     = errors.New("server has not been started")
	ErrServerAlreadyStarted = errors.New("server is already started")
)

// EmbeddedPostgres maintains all configuration and runtime functions for maintaining the lifecycle of one Postgres process.
type EmbeddedPostgres struct {
	config              Config
	cacheLocator        CacheLocator
	remoteFetchStrategy RemoteFetchStrategy
	initDatabase        initDatabase
	createDatabase      createDatabase
	started             bool
	syncedLogger        *syncedLogger
}

// NewDatabase creates a new EmbeddedPostgres struct that can be used to start and stop a Postgres process.
// When called with no parameters it will assume a default configuration state provided by the DefaultConfig method.
// When called with parameters the first Config parameter will be used for configuration.
func NewDatabase(config ...Config) *EmbeddedPostgres {
	if len(config) < 1 {
		return newDatabaseWithConfig(DefaultConfig())
	}

	return newDatabaseWithConfig(config[0])
}

func newDatabaseWithConfig(config Config) *EmbeddedPostgres {
	versionStrategy := defaultVersionStrategy(
		config,
		runtime.GOOS,
		runtime.GOARCH,
		linuxMachineName,
		shouldUseAlpineLinuxBuild,
	)
	cacheLocator := defaultCacheLocator(config.cachePath, versionStrategy)
	remoteFetchStrategy := defaultRemoteFetchStrategy(config.binaryRepositoryURL, versionStrategy, cacheLocator)

	return &EmbeddedPostgres{
		config:              config,
		cacheLocator:        cacheLocator,
		remoteFetchStrategy: remoteFetchStrategy,
		initDatabase:        defaultInitDatabase,
		createDatabase:      defaultCreateDatabase,
		started:             false,
	}
}

// Start will try to start the configured Postgres process returning an error when there were any problems with invocation.
// If any error occurs Start will try to also Stop the Postgres process in order to not leave any sub-process running.
//
//nolint:funlen
func (ep *EmbeddedPostgres) Start() error {
	if ep.started {
		return ErrServerAlreadyStarted
	}

	if !ep.config.useUnixSocket {
		if err := ensurePortAvailable(ep.config.port); err != nil {
			return err
		}
	}

	logger, err := newSyncedLogger(ep.config.logDirectory, ep.config.logger)
	if err != nil {
		return errors.New("unable to create logger")
	}

	ep.syncedLogger = logger

	cacheLocation, cacheExists := ep.cacheLocator()

	if ep.config.runtimePath == "" {
		ep.config.runtimePath = filepath.Join(filepath.Dir(cacheLocation), "extracted")
	}

	if ep.config.dataPath == "" {
		ep.config.dataPath = filepath.Join(ep.config.runtimePath, "data")
	}

	if err := os.RemoveAll(ep.config.runtimePath); err != nil {
		return fmt.Errorf("unable to clean up runtime directory %s with error: %s", ep.config.runtimePath, err)
	}

	if ep.config.binariesPath == "" {
		ep.config.binariesPath = ep.config.runtimePath
	}

	ep.logf(
		"embedded postgres binary setup cache=%s cache_exists=%t runtime_path=%s binaries_path=%s data_path=%s",
		cacheLocation,
		cacheExists,
		ep.config.runtimePath,
		ep.config.binariesPath,
		ep.config.dataPath,
	)

	if err := ep.downloadAndExtractBinary(cacheExists, cacheLocation); err != nil {
		return err
	}

	if err := os.MkdirAll(ep.config.runtimePath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create runtime directory %s with error: %s", ep.config.runtimePath, err)
	}

	reuseData := dataDirIsValid(ep.config.dataPath, ep.config.version)

	if !reuseData {
		if err := ep.cleanDataDirectoryAndInit(); err != nil {
			return err
		}
	}

	if err := startPostgres(ep); err != nil {
		return err
	}

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	ep.started = true

	if !reuseData {
		host := "localhost"
		if ep.config.useUnixSocket {
			host = ep.config.unixSocketDirectory
		}

		if err := ep.createDatabase(host, ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
			if stopErr := stopPostgres(ep); stopErr != nil {
				return fmt.Errorf("unable to stop database caused by error %s", err)
			}

			return err
		}
	}

	if err := healthCheckDatabaseOrTimeout(ep.config); err != nil {
		if stopErr := stopPostgres(ep); stopErr != nil {
			return fmt.Errorf("unable to stop database caused by error %s", err)
		}

		return err
	}

	return nil
}

func (ep *EmbeddedPostgres) downloadAndExtractBinary(cacheExists bool, cacheLocation string) error {
	// lock to prevent collisions with duplicate downloads
	mu.Lock()
	defer mu.Unlock()

	pgCtlPath := filepath.Join(ep.config.binariesPath, "bin", "pg_ctl")
	_, binDirErr := os.Stat(pgCtlPath)
	if os.IsNotExist(binDirErr) {
		if !cacheExists {
			ep.logf("downloading embedded postgres archive cache=%s", cacheLocation)
			if err := ep.remoteFetchStrategy(ep.logf); err != nil {
				return err
			}
		} else {
			ep.logf("using cached embedded postgres archive cache=%s", cacheLocation)
		}

		ep.logf("extracting embedded postgres archive archive=%s destination=%s", cacheLocation, ep.config.binariesPath)
		if err := decompressTarXz(defaultTarReader, cacheLocation, ep.config.binariesPath, ep.logf); err != nil {
			return err
		}
		ep.logf("embedded postgres archive extracted archive=%s destination=%s", cacheLocation, ep.config.binariesPath)
	} else {
		ep.logf("embedded postgres binaries already available pg_ctl=%s", pgCtlPath)
	}
	return nil
}

func (ep *EmbeddedPostgres) logf(format string, args ...any) {
	if ep == nil || ep.syncedLogger == nil {
		return
	}
	ep.syncedLogger.logf(format, args...)
}

func (ep *EmbeddedPostgres) GetConnectionURL() string {
	return ep.config.GetConnectionURL()
}

func (ep *EmbeddedPostgres) cleanDataDirectoryAndInit() error {
	if err := os.RemoveAll(ep.config.dataPath); err != nil {
		return fmt.Errorf("unable to clean up data directory %s with error: %s", ep.config.dataPath, err)
	}

	if err := ep.initDatabase(ep.config.binariesPath, ep.config.runtimePath, ep.config.dataPath, ep.config.username, ep.config.password, ep.config.locale, ep.config.encoding, ep.syncedLogger.file); err != nil {
		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return ErrServerNotStarted
	}

	if err := stopPostgres(ep); err != nil {
		return err
	}

	ep.started = false

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	return nil
}

func encodeOptions(port uint32, parameters map[string]string) string {
	options := []string{fmt.Sprintf("-p %d", port)}
	for k, v := range parameters {
		// Double-quote parameter values - they may have spaces.
		// Careful: CMD on Windows uses only double quotes to delimit strings.
		// It treats single quotes as regular characters.
		options = append(options, fmt.Sprintf("-c %s=\"%s\"", k, v))
	}
	return strings.Join(options, " ")
}

func startPostgres(ep *EmbeddedPostgres) error {
	if err := ensureFreeBSDRuntimeUser(ep.config.binariesPath, ep.config.runtimePath, ep.config.dataPath); err != nil {
		return err
	}

	if ep.config.startParameters == nil {
		ep.config.startParameters = make(map[string]string)
	}

	if ep.config.useUnixSocket {
		ep.config.startParameters["listen_addresses"] = ""
		ep.config.startParameters["unix_socket_directories"] = ep.config.unixSocketDirectory
	}

	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	postgresProcess := wrapCommandForRuntimeUser(exec.Command(postgresBinary, "start", "-w",
		"-D", ep.config.dataPath,
		"-o", encodeOptions(ep.config.port, ep.config.startParameters)))
	postgresProcess.Stdout = ep.syncedLogger.file
	postgresProcess.Stderr = ep.syncedLogger.file

	if err := postgresProcess.Run(); err != nil {
		_ = ep.syncedLogger.flush()
		logContent, _ := readLogsOrTimeout(ep.syncedLogger.file)
		logText := string(logContent)

		if runtime.GOOS == "freebsd" && needsFreeBSDICUCopyHint(logText) {
			logText += freeBSDICUCopyHint(ep.config.binariesPath)
		}

		return fmt.Errorf("could not start postgres using %s:\n%s", postgresProcess.String(), logText)
	}

	return nil
}

func needsFreeBSDICUCopyHint(logText string) bool {
	return strings.Contains(logText, `U_FILE_ACCESS_ERROR`) ||
		strings.Contains(logText, `could not open collator for locale "und"`) ||
		strings.Contains(logText, `pg_collation_actual_version(oid)`) ||
		strings.Contains(logText, `icu`)
}

func freeBSDICUCopyHint(binariesPath string) string {
	icuRoot := filepath.Join(binariesPath, "share", "icu")
	return fmt.Sprintf(
		"\nFreeBSD ICU hint:\n"+
			"  The embedded bundle ships ICU data under %s\n"+
			"  If PostgreSQL still fails to load collations, copy the bundled ICU version directory into /usr/local/share/icu/.\n"+
			"  Example:\n"+
			"    cp -R %s/* /usr/local/share/icu/\n",
		icuRoot,
		icuRoot,
	)
}

func stopPostgres(ep *EmbeddedPostgres) error {
	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	postgresProcess := wrapCommandForRuntimeUser(exec.Command(postgresBinary, "stop", "-w",
		"-D", ep.config.dataPath))
	postgresProcess.Stderr = ep.syncedLogger.file
	postgresProcess.Stdout = ep.syncedLogger.file

	if err := postgresProcess.Run(); err != nil {
		return err
	}

	return nil
}

func ensurePortAvailable(port uint32) error {
	conn, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("process already listening on port %d", port)
	}

	if err := conn.Close(); err != nil {
		return err
	}

	return nil
}

func dataDirIsValid(dataDir string, version PostgresVersion) bool {
	pgVersion := filepath.Join(dataDir, "PG_VERSION")

	d, err := os.ReadFile(pgVersion)
	if err != nil {
		return false
	}

	v := strings.TrimSuffix(string(d), "\n")

	return strings.HasPrefix(string(version), v)
}
