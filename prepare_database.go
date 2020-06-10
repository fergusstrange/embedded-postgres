package embeddedpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lib/pq"
)

type initDatabase func(binaryExtractLocation, username, password, locale string) error
type createDatabase func(port uint32, username, password, database string) error

func defaultInitDatabase(binaryExtractLocation, username, password, locale string) error {
	passwordFile, err := createPasswordFile(binaryExtractLocation, password)
	if err != nil {
		return err
	}

	args := []string{
		"-A", "password",
		"-U", username,
		"-D", filepath.Join(binaryExtractLocation, "data"),
		fmt.Sprintf("--pwfile=%s", passwordFile),
	}

	if locale != "" {
		args = append(args, fmt.Sprintf("--locale=%s", locale))
	}

	postgresInitDbBinary := filepath.Join(binaryExtractLocation, "bin/initdb")
	postgresInitDbProcess := exec.Command(postgresInitDbBinary, args...)
	postgresInitDbProcess.Stderr = os.Stderr
	postgresInitDbProcess.Stdout = os.Stdout

	if err := postgresInitDbProcess.Run(); err != nil {
		return fmt.Errorf("unable to init database using: %s", postgresInitDbProcess.String())
	}

	return nil
}

func createPasswordFile(binaryExtractLocation, password string) (string, error) {
	passwordFileLocation := filepath.Join(binaryExtractLocation, "pwfile")
	if err := ioutil.WriteFile(passwordFileLocation, []byte(password), 0600); err != nil {
		return "", fmt.Errorf("unable to write password file to %s", passwordFileLocation)
	}

	return passwordFileLocation, nil
}

func defaultCreateDatabase(port uint32, username, password, database string) error {
	if database == "postgres" {
		return nil
	}

	conn, err := openDatabaseConnection(port, username, password, "postgres")
	if err != nil {
		return errorCustomDatabase(database, err)
	}

	if _, err := sql.OpenDB(conn).Exec(fmt.Sprintf("CREATE DATABASE %s", database)); err != nil {
		return errorCustomDatabase(database, err)
	}

	return nil
}

func healthCheckDatabaseOrTimeout(config Config) error {
	healthCheckSignal := make(chan bool)

	defer close(healthCheckSignal)

	timeout, cancelFunc := context.WithTimeout(context.Background(), config.startTimeout)

	defer cancelFunc()

	go func() {
		for timeout.Err() == nil {
			if err := healthCheckDatabase(config.port, config.database, config.username, config.password); err != nil {
				continue
			}
			healthCheckSignal <- true
			break
		}
	}()

	select {
	case <-healthCheckSignal:
		return nil
	case <-timeout.Done():
		return errors.New("timed out waiting for database to become available")
	}
}

func healthCheckDatabase(port uint32, database, username, password string) error {
	conn, err := openDatabaseConnection(port, username, password, database)
	if err != nil {
		return err
	}

	if _, err := sql.OpenDB(conn).Query("SELECT 1"); err != nil {
		return err
	}

	return nil
}

func openDatabaseConnection(port uint32, username string, password string, database string) (*pq.Connector, error) {
	conn, err := pq.NewConnector(fmt.Sprintf("host=localhost port=%d user=%s password=%s dbname=%s sslmode=disable",
		port,
		username,
		password,
		database))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func errorCustomDatabase(database string, err error) error {
	return fmt.Errorf("unable to connect to create database with custom name %s with the following error: %s", database, err)
}
