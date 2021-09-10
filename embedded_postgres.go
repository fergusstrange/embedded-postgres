package embeddedpostgres

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mholt/archiver/v3"
)

// EmbeddedPostgres maintains all configuration and runtime functions for maintaining the lifecycle of one Postgres process.
type EmbeddedPostgres struct {
	config              Config
	cacheLocator        CacheLocator
	remoteFetchStrategy RemoteFetchStrategy
	initDatabase        initDatabase
	createDatabase      createDatabase
	started             bool
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
	remoteFetchStrategy := defaultRemoteFetchStrategy("https://repo1.maven.org", versionStrategy, cacheLocator)

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
//nolint:funlen
func (ep *EmbeddedPostgres) Start() error {
	if ep.started {
		return errors.New("server is already started")
	}

	if err := ensurePortAvailable(ep.config.port); err != nil {
		return err
	}

	cacheLocation, exists := ep.cacheLocator()
	if !exists {
		if err := ep.remoteFetchStrategy(); err != nil {
			return err
		}
	}

	if ep.config.runtimePath == "" {
		ep.config.runtimePath = filepath.Join(filepath.Dir(cacheLocation), "extracted")
	}
	if ep.config.dataPath == "" {
		ep.config.dataPath = filepath.Join(ep.config.runtimePath, "data")
	}

	if err := os.RemoveAll(ep.config.runtimePath); err != nil {
		return fmt.Errorf("unable to clean up runtime directory %s with error: %s", ep.config.runtimePath, err)
	}

	if err := archiver.NewTarXz().Unarchive(cacheLocation, ep.config.runtimePath); err != nil {
		return fmt.Errorf("unable to extract postgres archive %s to %s", cacheLocation, ep.config.runtimePath)
	}

	reuseData := dataDirIsValid(ep.config.dataPath, ep.config.version)

	if !reuseData {
		if err := os.RemoveAll(ep.config.dataPath); err != nil {
			return fmt.Errorf("unable to clean up data directory %s with error: %s", ep.config.dataPath, err)
		}

		if err := ep.initDatabase(ep.config.runtimePath, ep.config.runtimePath, ep.config.dataPath, ep.config.username, ep.config.password, ep.config.locale, ep.config.logger); err != nil {
			return err
		}
	}

	if err := startPostgres(ep.config); err != nil {
		return err
	}

	ep.started = true

	if !reuseData {
		if err := ep.createDatabase(ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
			if stopErr := stopPostgres(ep.config); stopErr != nil {
				return fmt.Errorf("unable to stop database casused by error %s", err)
			}

			return err
		}
	}

	if err := healthCheckDatabaseOrTimeout(ep.config); err != nil {
		if stopErr := stopPostgres(ep.config); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return errors.New("server has not been started")
	}

	if err := stopPostgres(ep.config); err != nil {
		return err
	}

	ep.started = false

	return nil
}

func startPostgres(config Config) error {
	postgresBinary := filepath.Join(config.runtimePath, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "start", "-w",
		"-D", config.dataPath,
		"-o", fmt.Sprintf(`"-p %d"`, config.port))
	postgresProcess.Stderr = config.logger
	postgresProcess.Stdout = config.logger

	if err := postgresProcess.Run(); err != nil {
		return fmt.Errorf("could not start postgres using %s", postgresProcess.String())
	}

	return nil
}

func stopPostgres(config Config) error {
	postgresBinary := filepath.Join(config.runtimePath, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "stop", "-w",
		"-D", config.dataPath)
	postgresProcess.Stderr = config.logger
	postgresProcess.Stdout = config.logger

	return postgresProcess.Run()
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
