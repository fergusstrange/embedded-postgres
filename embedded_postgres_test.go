package embeddedpostgres

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultConfig(t *testing.T) {
	database := NewDatabase()
	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"))
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err = db.Ping(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}

func Test_ErrorWhenPortAlreadyTaken(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:9887")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := listener.Close(); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().
		Port(9887))

	err = database.Start()

	assert.EqualError(t, err, "process already listening on port 9887")
}

func Test_ErrorWhenRemoteFetchError(t *testing.T) {
	database := NewDatabase()
	database.cacheLocator = func() (string, bool) {
		return "", false
	}
	database.remoteFetchStrategy = func() error {
		return errors.New("did not work")
	}

	err := database.Start()

	assert.EqualError(t, err, "did not work")
}

func Test_ErrorWhenUnableToUnArchiveFile_WrongFormat(t *testing.T) {
	jarFile, cleanUp := createTempZipArchive()
	defer cleanUp()

	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
		StartTimeout(10 * time.Second))

	database.cacheLocator = func() (string, bool) {
		return jarFile, true
	}

	err := database.Start()

	if err == nil {
		if err := database.Stop(); err != nil {
			panic(err)
		}
	}

	assert.EqualError(t, err, fmt.Sprintf("unable to extract postgres archive %s to %s", jarFile, filepath.Join(filepath.Dir(jarFile), "extracted")))
}

func Test_ErrorWhenUnableToInitDatabase(t *testing.T) {
	jarFile, cleanUp := createTempXzArchive()
	defer cleanUp()

	extractPath, err := ioutil.TempDir(filepath.Dir(jarFile), "extract")
	if err != nil {
		panic(err)
	}

	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
		RuntimePath(extractPath).
		StartTimeout(10 * time.Second))

	database.cacheLocator = func() (string, bool) {
		return jarFile, true
	}

	database.initDatabase = func(binaryExtractLocation, username, password string) error {
		return errors.New("ah it did not work")
	}

	err = database.Start()

	if err == nil {
		if err := database.Stop(); err != nil {
			panic(err)
		}
	}

	assert.EqualError(t, err, "ah it did not work")
}

func Test_ErrorWhenUnableToCreateDatabase(t *testing.T) {
	jarFile, cleanUp := createTempXzArchive()

	defer cleanUp()

	extractPath, err := ioutil.TempDir(filepath.Dir(jarFile), "extract")

	if err != nil {
		panic(err)
	}

	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
		RuntimePath(extractPath).
		StartTimeout(10 * time.Second))

	database.createDatabase = func(port uint32, username, password, database string) error {
		return errors.New("ah noes")
	}

	err = database.Start()

	if err == nil {
		if err := database.Stop(); err != nil {
			panic(err)
		}
	}

	assert.EqualError(t, err, "ah noes")
}

func Test_TimesOutWhenCannotStart(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		Database("something-fancy").
		StartTimeout(500 * time.Millisecond))

	database.createDatabase = func(port uint32, username, password, database string) error {
		return nil
	}

	err := database.Start()

	assert.EqualError(t, err, "timed out waiting for database to become available")
}

func Test_ErrorWhenStopCalledBeforeStart(t *testing.T) {
	database := NewDatabase()

	err := database.Stop()

	assert.EqualError(t, err, "server has not been started")
}

func Test_ErrorWhenStartCalledWhenAlreadyStarted(t *testing.T) {
	database := NewDatabase()

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	err := database.Start()
	assert.NoError(t, err)

	err = database.Start()
	assert.EqualError(t, err, "server is already started")
}

func Test_ErrorWhenCannotStartPostgresProcess(t *testing.T) {
	jarFile, cleanUp := createTempXzArchive()

	defer cleanUp()

	extractPath, err := ioutil.TempDir(filepath.Dir(jarFile), "extract")
	if err != nil {
		panic(err)
	}

	database := NewDatabase(DefaultConfig().
		RuntimePath(extractPath))

	database.cacheLocator = func() (string, bool) {
		return jarFile, true
	}

	database.initDatabase = func(binaryExtractLocation, username, password string) error {
		return nil
	}

	err = database.Start()

	assert.EqualError(t, err, fmt.Sprintf(`could not start postgres using %s/bin/pg_ctl start -w -D %s/data -o "-p 5432"`, extractPath, extractPath))
}

func Test_CustomConfig(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "embedded_postgres_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
		Version(V12).
		RuntimePath(tempDir).
		Port(9876).
		StartTimeout(10 * time.Second))
	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=9876 user=gin password=wine dbname=beer sslmode=disable"))
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err = db.Ping(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}

func Test_CanStartAndStopTwice(t *testing.T) {
	database := NewDatabase()

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"))
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err = db.Ping(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err = sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"))
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err = db.Ping(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}
