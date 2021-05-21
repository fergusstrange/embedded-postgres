package embeddedpostgres

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
		isAlpineLinux,
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

	binaryExtractLocation := userRuntimePathOrDefault(ep.config.runtimePath, cacheLocation)
	if err := os.RemoveAll(binaryExtractLocation); err != nil {
		return fmt.Errorf("unable to clean up runtime directory %s with error: %s", binaryExtractLocation, err)
	}

	if err := archiver.NewTarXz().Unarchive(cacheLocation, binaryExtractLocation); err != nil {
		return fmt.Errorf("unable to extract postgres archive %s to %s", cacheLocation, binaryExtractLocation)
	}

	dataLocation := userDataPathOrDefault(ep.config.dataPath, binaryExtractLocation)

	reuseData := ep.config.dataPath != "" && dataDirIsValid(dataLocation, ep.config.version)

	if !reuseData {
		if err := os.RemoveAll(dataLocation); err != nil {
			return fmt.Errorf("unable to clean up data directory %s with error: %s", dataLocation, err)
		}

		if err := ep.initDatabase(binaryExtractLocation, dataLocation, ep.config.username, ep.config.password, ep.config.locale, ep.config.logger); err != nil {
			return err
		}
	}

	if err := startPostgres(binaryExtractLocation, ep.config); err != nil {
		return err
	}

	ep.started = true

	if !reuseData {
		if err := ep.createDatabase(ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
			if stopErr := stopPostgres(binaryExtractLocation, ep.config); stopErr != nil {
				return fmt.Errorf("unable to stop database casused by error %s", err)
			}

			return err
		}
	}

	if err := healthCheckDatabaseOrTimeout(ep.config); err != nil {
		if stopErr := stopPostgres(binaryExtractLocation, ep.config); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	cacheLocation, exists := ep.cacheLocator()
	if !exists || !ep.started {
		return errors.New("server has not been started")
	}

	binaryExtractLocation := userRuntimePathOrDefault(ep.config.runtimePath, cacheLocation)
	if err := stopPostgres(binaryExtractLocation, ep.config); err != nil {
		return err
	}

	ep.started = false

	return nil
}

func startPostgres(binaryExtractLocation string, config Config) error {
	postgresBinary := filepath.Join(binaryExtractLocation, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "start", "-w",
		"-D", userDataPathOrDefault(config.dataPath, binaryExtractLocation),
		"-o", fmt.Sprintf(`"-p %d"`, config.port))
	log.Println(postgresProcess.String())
	postgresProcess.Stderr = config.logger
	postgresProcess.Stdout = config.logger

	if err := postgresProcess.Run(); err != nil {
		return fmt.Errorf("could not start postgres using %s", postgresProcess.String())
	}

	return nil
}

func stopPostgres(binaryExtractLocation string, config Config) error {
	postgresBinary := filepath.Join(binaryExtractLocation, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "stop", "-w",
		"-D", userDataPathOrDefault(config.dataPath, binaryExtractLocation))
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

func userRuntimePathOrDefault(userLocation, cacheLocation string) string {
	if userLocation != "" {
		return userLocation
	}

	return filepath.Join(filepath.Dir(cacheLocation), "extracted")
}

func userDataPathOrDefault(userLocation, runtimeLocation string) string {
	if userLocation != "" {
		return userLocation
	}

	return filepath.Join(runtimeLocation, "data")
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
