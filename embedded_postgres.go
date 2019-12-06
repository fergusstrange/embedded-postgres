package embeddedpostgres

import (
	"errors"
	"fmt"
	"github.com/mholt/archiver"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

type EmbeddedPostgres struct {
	config              Config
	cacheLocator        CacheLocator
	remoteFetchStrategy RemoteFetchStrategy
	initDatabase        InitDatabase
	createDatabase      CreateDatabase
	started             bool
}

func NewDatabase(config ...Config) *EmbeddedPostgres {
	if len(config) < 1 {
		return newDatabaseWithConfig(DefaultConfig())
	}
	return newDatabaseWithConfig(config[0])
}

func newDatabaseWithConfig(config Config) *EmbeddedPostgres {
	versionStrategy := defaultVersionStrategy(config)
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

	binaryExtractLocation := userLocationOrDefault(ep.config.runtimePath, cacheLocation)
	if err := os.RemoveAll(binaryExtractLocation); err != nil {
		return fmt.Errorf("unable to clean up directory %s with error: %s", binaryExtractLocation, err)
	}

	if err := archiver.NewTarXz().Unarchive(cacheLocation, binaryExtractLocation); err != nil {
		return fmt.Errorf("unable to extract postgres archive %s to %s", cacheLocation, binaryExtractLocation)
	}

	if err := ep.initDatabase(binaryExtractLocation, ep.config.username, ep.config.password); err != nil {
		return err
	}

	if err := startPostgres(binaryExtractLocation, ep.config); err != nil {
		return err
	}

	ep.started = true

	if err := ep.createDatabase(ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
		if stopErr := stopPostgres(binaryExtractLocation); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}
		return err
	}

	if err := healthCheckDatabaseOrTimeout(ep.config); err != nil {
		if stopErr := stopPostgres(binaryExtractLocation); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}
		return err
	}

	return nil
}

func (ep *EmbeddedPostgres) Stop() error {
	cacheLocation, exists := ep.cacheLocator()
	if !exists || !ep.started {
		return errors.New("server has not been started")
	}
	binaryExtractLocation := userLocationOrDefault(ep.config.runtimePath, cacheLocation)
	if err := stopPostgres(binaryExtractLocation); err != nil {
		return err
	}
	ep.started = false
	return nil
}

func startPostgres(binaryExtractLocation string, config Config) error {
	postgresBinary := filepath.Join(binaryExtractLocation, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "start", "-w",
		"-D", filepath.Join(binaryExtractLocation, "data"),
		"-o", fmt.Sprintf(`"-p %d"`, config.port))
	log.Println(postgresProcess.String())
	postgresProcess.Stderr = os.Stderr
	postgresProcess.Stdout = os.Stdout
	if err := postgresProcess.Run(); err != nil {
		return fmt.Errorf("could not start postgres using %s", postgresProcess.String())
	}
	return nil
}

func stopPostgres(binaryExtractLocation string) error {
	postgresBinary := filepath.Join(binaryExtractLocation, "bin/pg_ctl")
	postgresProcess := exec.Command(postgresBinary, "stop", "-w",
		"-D", filepath.Join(binaryExtractLocation, "data"))
	postgresProcess.Stderr = os.Stderr
	postgresProcess.Stdout = os.Stdout
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

func userLocationOrDefault(userLocation, cacheLocation string) string {
	if userLocation != "" {
		return userLocation
	}
	return filepath.Join(filepath.Dir(cacheLocation), "extracted")
}
