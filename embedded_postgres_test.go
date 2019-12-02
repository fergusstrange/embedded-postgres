package embeddedpostgres

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"
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

func Test_ErrorWhenUnableToUnArchiveFile(t *testing.T) {
	jarFile, cleanUp := createTempArchive()
	defer cleanUp()

	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
		RuntimePath("path_that_not_exists").
		Port(9876).
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

	assert.EqualError(t, err, fmt.Sprintf("unable to extract postgres archive %s to path_that_not_exists", jarFile))
}

func Test_TimesOutWhenCannotStart(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		StartTimeout(100 * time.Millisecond))
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

func shutdownDBAndFail(t *testing.T, err error, db *EmbeddedPostgres) {
	if err := db.Stop(); err != nil {
		t.Fatalf("Failed for version %s with error %s", db.config.version, err)
	}
	t.Fatalf("Failed for version %s with error %s", db.config.version, err)
}
