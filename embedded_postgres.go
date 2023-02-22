package embeddedpostgres

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	cacheLocator := defaultCacheLocator(versionStrategy)
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
		return errors.New("server is already started")
	}

	if ep.config.useUnixSocket == "" {
		if err := ensurePortAvailable(ep.config.port); err != nil {
			return err
		}
	}

	logger, err := newSyncedLogger("", ep.config.logger)
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

	_, binDirErr := os.Stat(filepath.Join(ep.config.binariesPath, "bin"))
	if os.IsNotExist(binDirErr) {
		if !cacheExists {
			if err := ep.remoteFetchStrategy(); err != nil {
				return err
			}
		}

		if err := decompressTarXz(defaultTarReader, cacheLocation, ep.config.binariesPath); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(ep.config.runtimePath, 0755); err != nil {
		return fmt.Errorf("unable to create runtime directory %s with error: %s", ep.config.runtimePath, err)
	}

	reuseData := dataDirIsValid(ep.config.dataPath, ep.config.version)

	if !reuseData {
		if err := ep.cleanDataDirectoryAndInit(); err != nil {
			return err
		}
	}

	// In case it is already running, try to stop it.
	_ = stopPostgres(ep)
	if err := startPostgres(ep); err != nil {
		return err
	}

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	ep.started = true

	if !reuseData {
		host := "localhost"
		if ep.config.useUnixSocket != "" {
			host = ep.config.useUnixSocket
		}
		if err := ep.createDatabase(host, ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
			if stopErr := stopPostgres(ep); stopErr != nil {
				return fmt.Errorf("unable to stop database casused by error %s", err)
			}

			return err
		}
	}

	if err := healthCheckDatabaseOrTimeout(ep.config); err != nil {
		if stopErr := stopPostgres(ep); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

		return err
	}

	return nil
}

func (ep *EmbeddedPostgres) cleanDataDirectoryAndInit() error {
	if err := os.RemoveAll(ep.config.dataPath); err != nil {
		return fmt.Errorf("unable to clean up data directory %s with error: %s", ep.config.dataPath, err)
	}

	c := ep.config
	if err := ep.initDatabase(c.binariesPath, c.runtimePath, c.dataPath, c.username, c.password, c.locale, c.useUnixSocket, ep.syncedLogger.file); err != nil {
		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return errors.New("server has not been started")
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

func startPostgres(ep *EmbeddedPostgres) error {
	// We don't use pg_ctl since that starts postgres in the background. We want
	// postgres to die if this process dies.

	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/postgres")
	postgresProcess := exec.Command(postgresBinary,
		"-D", ep.config.dataPath,
		"-p", strconv.Itoa(int(ep.config.port)))
	postgresProcess.Stdout = ep.syncedLogger.file
	postgresProcess.Stderr = ep.syncedLogger.file

	// We open stdin so that when this process dies postgres will get a signal.
	stdin, err := postgresProcess.StdinPipe()
	if err != nil {
		return err
	}

	if err := postgresProcess.Start(); err != nil {
		return fmt.Errorf("could not start postgres using %s: %w", postgresProcess.String(), err)
	}

	waitErrC := make(chan error, 1)
	go func() {
		defer stdin.Close()
		err := postgresProcess.Wait()
		waitErrC <- err
		if err != nil {
			_, _ = fmt.Fprintf(ep.syncedLogger.file, "%v embedded-postgres process exited with non-zero exit code: %v\n", time.Now(), err)
		}
	}()

	// Wait for pg_ctl to report happy news
	defaultWait := 60 * time.Second // mirrors pg_ctl's DEFAULT_WAIT
	deadline := time.Now().Add(defaultWait)
	for time.Now().Before(deadline) {
		if isReadyPostgres(ep) {
			// Success
			return nil
		}

		select {
		case err := <-waitErrC:
			return fmt.Errorf("could not start postgres using %s: %w", postgresProcess.String(), err)
		case <-time.After(time.Second / 10): // mirrors pg_ctl's WAITS_PER_SEC
			// try again
		}
	}

	// Failed to start, best-effort kill process and return an error
	_ = stdin.Close()
	_ = postgresProcess.Process.Kill()
	return fmt.Errorf("postgres failed to start after %v using %s", defaultWait, postgresProcess.String())
}

func isReadyPostgres(ep *EmbeddedPostgres) bool {
	pgCtl := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	if exec.Command(pgCtl, "-D", ep.config.dataPath, "status").Run() != nil {
		return false
	}

	// pg_ctl status returning success isn't enough, it just checks if the
	// postmaster PID is running. To be equivalent to pg_ctl start we also need
	// to check the status of the postmaster is ready. Without this check
	// queries will fail at first.
	//
	// The format of this PID file has been stable since Postgres 10.
	// https://sourcegraph.com/github.com/postgres/postgres@REL_15_2/-/blob/src/include/utils/pidfile.h?L44
	linePMStatus := 8
	pmStatusReady := "ready   "
	b, err := os.ReadFile(filepath.Join(ep.config.dataPath, "postmaster.pid"))
	if err != nil {
		return false
	}
	lines := bytes.Split(b, []byte("\n")) // pg_ctl readline only considers \n for newline (never \r\n)
	return len(lines) >= linePMStatus && bytes.Equal(lines[linePMStatus-1], []byte(pmStatusReady))
}

func stopPostgres(ep *EmbeddedPostgres) error {
	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "stop", "-w",
		"-D", ep.config.dataPath)
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

	d, err := ioutil.ReadFile(pgVersion)
	if err != nil {
		return false
	}

	v := strings.TrimSuffix(string(d), "\n")

	return strings.HasPrefix(string(version), v)
}
