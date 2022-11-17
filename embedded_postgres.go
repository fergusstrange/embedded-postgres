package embeddedpostgres

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
	cmd                 *exec.Cmd
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

	if err := ensurePortAvailable(ep.config.port); err != nil {
		return err
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

	ctx, cancelCtx := context.WithTimeout(context.Background(), ep.config.startTimeout)
	defer cancelCtx()

	if err = ep.startPostgres(ctx); err != nil {
		return err
	}

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	ep.started = true

	if !reuseData {
		if err := ep.createDatabase(ctx, ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
			if stopErr := ep.Stop(); stopErr != nil {
				return fmt.Errorf("unable to stop database casused by error %s", err)
			}

			return err
		}
	}

	if err := healthCheckDatabaseOrTimeout(ctx, ep.config); err != nil {
		if stopErr := ep.Stop(); stopErr != nil {
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

	if err := ep.initDatabase(ep.config.binariesPath, ep.config.runtimePath, ep.config.dataPath, ep.config.username, ep.config.password, ep.config.locale, ep.syncedLogger.file); err != nil {
		_ = ep.syncedLogger.flush()
		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return errors.New("server has not been started")
	}

	_ = ep.cmd.Process.Signal(syscall.SIGINT)
	_ = ep.cmd.Wait()

	ep.started = false

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	return nil
}

func (ep *EmbeddedPostgres) startPostgres(ctx context.Context) error {
	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/postgres")
	command := exec.Command(postgresBinary,
		"-D", ep.config.dataPath,
		"-p", fmt.Sprintf("%d", ep.config.port))
	command.Stdout = ep.syncedLogger.file
	command.Stderr = ep.syncedLogger.file

	if err := command.Start(); err != nil {
		return fmt.Errorf("could not start postgres using %s", command.String())
	}

	ep.cmd = command

	if err := ep.waitForPostmasterReady(ctx, 100*time.Millisecond); err != nil {
		if stopErr := command.Process.Signal(syscall.SIGINT); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

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

type pgStatus struct {
	Pid     int
	Running bool
	Output  string
}

func pgCtlStatus(config Config) (*pgStatus, error) {
	cmd := exec.Command(filepath.Join(config.binariesPath, "bin/pg_ctl"),
		"status",
		"-D",
		config.dataPath,
	)
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = buf

	err := cmd.Start()

	if err != nil {
		return nil, err
	}

	err = cmd.Wait()

	if err != nil {
		eErr := &exec.ExitError{}
		if !errors.As(err, &eErr) {
			return nil, fmt.Errorf("%s - %w: %s", cmd.String(), err, buf.String())
		}
	}

	status := &pgStatus{Output: buf.String()}

	// scanner to support windows
	// The first line of a good response will be
	// pg_ctl: server is running (PID: 12345)
	sc := bufio.NewScanner(buf)
	if sc.Scan() {
		line := sc.Text()
		if _, err := fmt.Sscanf(line, "pg_ctl: server is running (PID: %d)", &status.Pid); err != nil {
			return status, nil
		}

		status.Running = true

		return status, nil
	}

	return status, nil
}

func (ep *EmbeddedPostgres) waitForPostmasterReady(ctx context.Context, interval time.Duration) (err error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for database to become available: %w", err)
		case <-ticker.C:
			_ = ep.syncedLogger.flush()

			var status *pgStatus

			if ep.cmd.Process == nil {
				return fmt.Errorf("no process found")
			}

			status, err = pgCtlStatus(ep.config)

			if status != nil && status.Running {
				if status.Pid != ep.cmd.Process.Pid {
					return fmt.Errorf("process running, but for wrong pid, expected %d, got %d", ep.cmd.Process.Pid, status.Pid)
				}

				return nil
			}
		}
	}
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
