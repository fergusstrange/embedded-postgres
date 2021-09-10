package embeddedpostgres

import (
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
	binTempDir, err := ioutil.TempDir("", "prepare_database_test_bin")
	if err != nil {
		panic(err)
	}
	runtimeTempDir, err := ioutil.TempDir("", "prepare_database_test_runtime")
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
	}()

	err = defaultInitDatabase(binTempDir, runtimeTempDir, filepath.Join(runtimeTempDir, "data"), "Tom", "Beer", "", os.Stderr)

	assert.EqualError(t, err, fmt.Sprintf("unable to init database using: %s/bin/initdb -A password -U Tom -D %s/data --pwfile=%s/pwfile",
		binTempDir,
		runtimeTempDir,
		runtimeTempDir))
	assert.FileExists(t, filepath.Join(runtimeTempDir, "pwfile"))
}

func Test_defaultInitDatabase_ErrorInvalidLocaleSetting(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	err = defaultInitDatabase(tempDir, tempDir, filepath.Join(tempDir, "data"), "postgres", "postgres", "en_XY", os.Stderr)

	assert.EqualError(t, err, fmt.Sprintf("unable to init database using: %s/bin/initdb -A password -U postgres -D %s/data --pwfile=%s/pwfile --locale=en_XY",
		tempDir,
		tempDir,
		tempDir))
}

func Test_defaultInitDatabase_PwFileRemoved(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
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

func Test_defaultCreateDatabase_ErrorWhenQueryError(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		Database("b33r"))
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	err := defaultCreateDatabase(5432, "postgres", "postgres", "b33r")

	assert.EqualError(t, err, `unable to connect to create database with custom name b33r with the following error: pq: database "b33r" already exists`)
}

func Test_healthCheckDatabase_ErrorWhenSQLConnectingError(t *testing.T) {
	err := healthCheckDatabase(1234, "tom client_encoding=lol", "more", "b33r")

	assert.EqualError(t, err, "client_encoding must be absent or 'UTF8'")
}
