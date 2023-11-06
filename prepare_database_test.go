package embeddedpostgres

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultInitDatabase_ErrorWhenCannotCreatePasswordFile(t *testing.T) {
	err := defaultInitDatabase("path_not_exists", "path_not_exists", "path_not_exists", "Tom", "Beer", "", os.Stderr)

	assert.EqualError(t, err, "unable to write password file to path_not_exists/pwfile")
}

func Test_defaultInitDatabase_ErrorWhenCannotStartInitDBProcess(t *testing.T) {
	binTempDir, err := os.MkdirTemp("", "prepare_database_test_bin")
	if err != nil {
		panic(err)
	}

	runtimeTempDir, err := os.MkdirTemp("", "prepare_database_test_runtime")
	if err != nil {
		panic(err)
	}

	logFile, err := ioutil.TempFile("", "prepare_database_test_log")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(binTempDir); err != nil {
			panic(err)
		}

		if err := os.RemoveAll(runtimeTempDir); err != nil {
			panic(err)
		}

		if err := os.Remove(logFile.Name()); err != nil {
			panic(err)
		}
	}()

	_, _ = logFile.Write([]byte("and here are the logs!"))

	err = defaultInitDatabase(binTempDir, runtimeTempDir, filepath.Join(runtimeTempDir, "data"), "Tom", "Beer", "", logFile)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unable to init database using '%s/bin/initdb -A password -U Tom -D %s/data --pwfile=%s/pwfile'",
		binTempDir,
		runtimeTempDir,
		runtimeTempDir))
	assert.Contains(t, err.Error(), "and here are the logs!")
	assert.FileExists(t, filepath.Join(runtimeTempDir, "pwfile"))
}

func Test_defaultInitDatabase_ErrorInvalidLocaleSetting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	err = defaultInitDatabase(tempDir, tempDir, filepath.Join(tempDir, "data"), "postgres", "postgres", "en_XY", os.Stderr)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unable to init database using '%s/bin/initdb -A password -U postgres -D %s/data --pwfile=%s/pwfile --locale=en_XY'",
		tempDir,
		tempDir,
		tempDir))
}

func Test_defaultInitDatabase_PwFileRemoved(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().RuntimePath(tempDir))
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	pwFile := filepath.Join(tempDir, "pwfile")
	_, err = os.Stat(pwFile)

	assert.True(t, os.IsNotExist(err), "pwfile (%v) still exists after starting the db", pwFile)
}

func Test_defaultCreateDatabase_ErrorWhenSQLOpenError(t *testing.T) {
	err := defaultCreateDatabase(1234, "user client_encoding=lol", "password", "database")

	assert.EqualError(t, err, "unable to connect to create database with custom name database with the following error: client_encoding must be absent or 'UTF8'")
}

func Test_defaultCreateDatabase_DashesInName(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		Port(9832).
		Database("my-cool-database"))

	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	if err := database.Stop(); err != nil {
		t.Fatal(err)
	}
}

func Test_defaultCreateDatabase_ErrorWhenQueryError(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		Port(9831).
		Database("b33r"))
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	err := defaultCreateDatabase(9831, "postgres", "postgres", "b33r")

	assert.EqualError(t, err, `unable to connect to create database with custom name b33r with the following error: pq: database "b33r" already exists`)
}

func Test_healthCheckDatabase_ErrorWhenSQLConnectingError(t *testing.T) {
	err := healthCheckDatabase(1234, "tom client_encoding=lol", "more", "b33r")

	assert.EqualError(t, err, "client_encoding must be absent or 'UTF8'")
}

type CloserWithoutErr struct{}

func (c *CloserWithoutErr) Close() error {
	return nil
}

func TestConnCloserWithoutErr(t *testing.T) {
	originalErr := errors.New("OriginalError")

	tests := []struct {
		name           string
		err            error
		expectedErrTxt string
	}{
		{
			"No original error, no error from closer",
			nil,
			"",
		},
		{
			"original error, no error from closer",
			originalErr,
			originalErr.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultErr := connectionClose(&CloserWithoutErr{}, tt.err)

			if len(tt.expectedErrTxt) == 0 {
				if resultErr != nil {
					t.Fatalf("Expected nil error, got error: %v", resultErr)
				}

				return
			}

			if resultErr.Error() != tt.expectedErrTxt {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectedErrTxt, resultErr)
			}
		})
	}
}

type CloserWithErr struct{}

const testError = "TestError"

func (c *CloserWithErr) Close() error {
	return errors.New(testError)
}

func TestConnCloserWithErr(t *testing.T) {
	originalErr := errors.New("OriginalError")

	closeDBConnErr := fmt.Errorf(fmtCloseDBConn, errors.New(testError))

	tests := []struct {
		name           string
		err            error
		expectedErrTxt string
	}{
		{
			"No original error, error from closer",
			nil,
			closeDBConnErr.Error(),
		},
		{
			"original error, error from closer",
			originalErr,
			fmt.Errorf(fmtAfterError, closeDBConnErr, originalErr).Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultErr := connectionClose(&CloserWithErr{}, tt.err)

			if len(tt.expectedErrTxt) == 0 {
				if resultErr != nil {
					t.Fatalf("Expected nil error, got error: %v", resultErr)
				}

				return
			}

			if resultErr.Error() != tt.expectedErrTxt {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectedErrTxt, resultErr)
			}
		})
	}
}
