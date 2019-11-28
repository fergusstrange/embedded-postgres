package embeddedpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/mholt/archiver"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type EmbeddedPostgres struct {
	config              Config
	cacheLocator        CacheLocator
	remoteFetchStrategy RemoteFetchStrategy
	startErrors         chan error
	stopErrors          chan error
	stopSignal          chan bool
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
	remoteFetchStrategy := defaultRemoteFetchStrategy(versionStrategy, cacheLocator)
	return &EmbeddedPostgres{
		config:              config,
		cacheLocator:        cacheLocator,
		remoteFetchStrategy: remoteFetchStrategy,
		startErrors:         make(chan error, 1),
		stopErrors:          make(chan error, 1),
		stopSignal:          make(chan bool, 1),
	}
}

func (ep *EmbeddedPostgres) Start() error {
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
		return fmt.Errorf("unable to extract postgres archive %s to %s with error: %s", cacheLocation, binaryExtractLocation, err)
	}

	if err := initDatabase(binaryExtractLocation, ep.config.username, ep.config.password); err != nil {
		return err
	}

	go startPostgres(binaryExtractLocation, ep.config, ep.stopSignal, ep.startErrors, ep.stopErrors)

	for err := range ep.startErrors {
		ep.stopSignal <- true
		close(ep.stopSignal)
		return err
	}

	if err := createDatabase(ep.config.port, ep.config.username, ep.config.password, ep.config.database); err != nil {
		return err
	}

	return nil
}

func (ep *EmbeddedPostgres) Stop() error {
	ep.stopSignal <- true
	close(ep.stopSignal)
	for err := range ep.stopErrors {
		return err
	}
	return nil
}

func startPostgres(binaryExtractLocation string, config Config, stopSignal chan bool, startErrors, stopErrors chan error) {
	postgresBinary := filepath.Join(binaryExtractLocation, "bin/postgres")
	postgresProcess := exec.Command(postgresBinary, "-p", fmt.Sprintf("%d", config.port), "-h", "localhost", "-D", filepath.Join(binaryExtractLocation, "data"))
	postgresProcess.Stderr = os.Stderr
	postgresProcess.Stdout = os.Stdout
	if err := postgresProcess.Start(); err != nil {
		startErrors <- err
		close(startErrors)
		return
	}

	if err := healthCheckDatabaseOrTimeout(config); err != nil {
		startErrors <- err
		close(startErrors)
		return
	}

	close(startErrors)

	for range stopSignal {
		if err := stopPostgres(postgresProcess); err != nil {
			stopErrors <- err
		}
		close(stopErrors)
	}
}

func stopPostgres(postgresProcess *exec.Cmd) error {
	if err := postgresProcess.Process.Signal(syscall.SIGQUIT); err != nil {
		return err
	}
	if err := postgresProcess.Wait(); err != nil {
		return err
	}
	return nil
}

func initDatabase(binaryExtractLocation, username, password string) error {
	passwordFile, err := createPasswordFile(binaryExtractLocation, password)
	if err != nil {
		return err
	}
	postgresInitDbBinary := filepath.Join(binaryExtractLocation, "bin/initdb")
	postgresInitDbProcess := exec.Command(postgresInitDbBinary,
		"-A", "password",
		"-U", username,
		"-D", filepath.Join(binaryExtractLocation, "data"),
		fmt.Sprintf("--pwfile=%s", passwordFile))
	postgresInitDbProcess.Stderr = os.Stderr
	postgresInitDbProcess.Stdout = os.Stdout
	if err := postgresInitDbProcess.Run(); err != nil {
		return fmt.Errorf("unable to init database with error: %s", err)
	}
	return nil
}

func createPasswordFile(binaryExtractLocation, password string) (string, error) {
	pwfileLocation := filepath.Join(binaryExtractLocation, "pwfile")
	if err := ioutil.WriteFile(pwfileLocation, []byte(password), 0600); err != nil {
		return "", fmt.Errorf("unable to write password file with error: %s", err)
	}
	return pwfileLocation, nil
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

func createDatabase(port uint32, username, password, database string) error {
	if database == "postgres" {
		return nil
	}
	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=%d user=%s password=%s dbname=%s sslmode=disable",
		port,
		username,
		password,
		"postgres"))
	if err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", database)); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}

	return nil
}

func healthCheckDatabaseOrTimeout(config Config) error {
	healthCheckSignal := make(chan bool)
	timeout, cancelFunc := context.WithTimeout(context.Background(), config.startTimeout)
	go func() {
		defer cancelFunc()
		defer close(healthCheckSignal)
		for timeout.Err() == nil {
			if err := healthCheckDatabase(config.port, config.username, config.password); err != nil {
				continue
			}
			healthCheckSignal <- true
		}
	}()
	select {
	case <-healthCheckSignal:
		return nil
	case <-timeout.Done():
		return errors.New("timed out waiting for database to start")
	}
}

func healthCheckDatabase(port uint32, username, password string) error {
	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=%d user=%s password=%s dbname=%s sslmode=disable",
		port,
		username,
		password,
		"postgres"))
	if err != nil {
		return err
	}
	rows, err := db.Query("SELECT 1")
	if err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if err := db.Close(); err != nil {
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
