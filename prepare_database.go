package embeddedpostgres

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lib/pq"
)

type initDatabase func(binaryExtractLocation, runtimePath, pgDataDir, username, password, locale string, logger *os.File) error
type createDatabase func(port uint32, username, password, database string) error

func defaultInitDatabase(binaryExtractLocation, runtimePath, pgDataDir, username, password, locale string, logger *os.File) error {
	passwordFile, err := createPasswordFile(runtimePath, password)
	if err != nil {
		return err
	}

	args := []string{
		"-A", "password",
		"-U", username,
		"-D", pgDataDir,
		fmt.Sprintf("--pwfile=%s", passwordFile),
	}

	if locale != "" {
		args = append(args, fmt.Sprintf("--locale=%s", locale))
	}

	postgresInitDBBinary := filepath.Join(binaryExtractLocation, "bin/initdb")
	postgresInitDBProcess := exec.Command(postgresInitDBBinary, args...)
	postgresInitDBProcess.Stderr = logger
	postgresInitDBProcess.Stdout = logger

	if err := postgresInitDBProcess.Run(); err != nil {
		return fmt.Errorf("unable to init database using: %s", postgresInitDBProcess.String())
	}

	if err = os.Remove(passwordFile); err != nil {
		return fmt.Errorf("unable to remove password file '%v'", passwordFile)
	}

	if err = enableTimescaleDB(pgDataDir); err != nil {
		return fmt.Errorf("unable to enable timescaledb library preloading: %w", err)
	}

	return nil
}

func enableTimescaleDB(pgDataDir string) (err error) {
	srcConfig := filepath.Join(pgDataDir, "postgresql.conf")
	destConfig := filepath.Join(pgDataDir, "postgresql.new")

	var input, output *os.File

	if input, output, err = openConfigFiles(srcConfig, destConfig); err != nil {
		return
	}

	scanner := bufio.NewScanner(input)
	if err = updatePgConfig(scanner, output); err != nil {
		return err
	}

	// We need to make sure both the input and output files are closed before we can rename the new file to replace the original
	if err = closeConfigFiles(input, output); err != nil {
		return
	}

	return os.Rename(destConfig, srcConfig)
}

func openConfigFiles(src, dst string) (input *os.File, output *os.File, err error) {
	if input, err = os.Open(src); err != nil {
		return nil, nil, fmt.Errorf("could not open postgresql.conf for reading: %w", err)
	}

	if output, err = os.Create(dst); err != nil {
		return nil, nil, fmt.Errorf("could not open new postgresql.conf for writing: %w", err)
	}

	return
}

func closeConfigFiles(input, output *os.File) (err error) {
	if err = input.Close(); err != nil {
		return fmt.Errorf("could not close postgresql.conf: %w", err)
	}

	if err = output.Close(); err != nil {
		return fmt.Errorf("could not close new postgresql.conf: %w", err)
	}

	return
}

func updatePgConfig(scanner *bufio.Scanner, output *os.File) (err error) {
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#shared_preload_libraries") {
			if err = writeNewConfig(output, "shared_preload_libraries = 'timescaledb'  # (change requires restart)"); err != nil {
				return
			}
			continue
		}

		if err = writeNewConfig(output, line); err != nil {
			return
		}
	}

	return
}

func writeNewConfig(output *os.File, line string) (err error) {
	if _, err = output.WriteString(fmt.Sprintf("%s\n", line)); err != nil {
		err = fmt.Errorf("could not write configs to new postgresql.conf file: %w", err)
		if closeErr := output.Close(); closeErr != nil {
			err = fmt.Errorf("could not close new postgresql.conf file: %w", err)
			return
		}
	}

	return
}

func createPasswordFile(runtimePath, password string) (string, error) {
	passwordFileLocation := filepath.Join(runtimePath, "pwfile")
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
