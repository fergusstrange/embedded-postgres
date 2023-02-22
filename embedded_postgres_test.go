package embeddedpostgres

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultConfig(t *testing.T) {
	defer verifyLeak(t)

	database := NewDatabase()
	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
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

	assert.EqualError(t, err, fmt.Sprintf(`unable to extract postgres archive %s to %s, if running parallel tests, configure RuntimePath to isolate testing directories`, jarFile, filepath.Join(filepath.Dir(jarFile), "extracted")))
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

	database.initDatabase = func(binaryExtractLocation, runtimePath, dataLocation, username, password, locale string, useUnixSocket string, logger *os.File) error {
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

	database.createDatabase = func(host string, port uint32, username, password, database string) error {
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

	database.createDatabase = func(host string, port uint32, username, password, database string) error {
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

	database.initDatabase = func(binaryExtractLocation, runtimePath, dataLocation, username, password, locale string, useUnixSocket string, logger *os.File) error {
		return nil
	}

	err = database.Start()

	assert.Contains(t, err.Error(), fmt.Sprintf(`could not start postgres using %s/bin/postgres -D %s/data -p 5432`, extractPath, extractPath))
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
		StartTimeout(10 * time.Second).
		Locale("C").
		Logger(nil))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=9876 user=gin password=wine dbname=beer sslmode=disable")
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

func Test_CustomLog(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "embedded_postgres_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	logger := customLogger{}

	database := NewDatabase(DefaultConfig().
		Logger(&logger))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
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

	current, err := user.Current()

	lines := strings.Split(string(logger.logLines), "\n")

	assert.NoError(t, err)
	assert.Contains(t, lines, fmt.Sprintf("The files belonging to this database system will be owned by user \"%s\".", current.Username))
	assert.Contains(t, lines, "syncing data to disk ... ok")
	assert.Contains(t, lines, "server stopped")
	assert.Less(t, len(lines), 55)
	assert.Greater(t, len(lines), 40)
}

func Test_CustomLocaleConfig(t *testing.T) {
	// C is the only locale we can guarantee to always work
	database := NewDatabase(DefaultConfig().Locale("C"))
	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
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

	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
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

	db, err = sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
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

//nolint:funlen
func Test_ReuseData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "embedded_postgres_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().DataPath(tempDir))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if _, err = db.Exec("CREATE TABLE test(id serial, value text, PRIMARY KEY(id))"); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if _, err = db.Exec("INSERT INTO test (value) VALUES ('foobar')"); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	database = NewDatabase(DefaultConfig().DataPath(tempDir))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err = sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if rows, err := db.Query("SELECT * FROM test"); err != nil {
		shutdownDBAndFail(t, err, database)
	} else {
		if !rows.Next() {
			shutdownDBAndFail(t, errors.New("no row from db"), database)
		}

		var (
			id    int64
			value string
		)
		if err := rows.Scan(&id, &value); err != nil {
			shutdownDBAndFail(t, err, database)
		}
		if value != "foobar" {
			shutdownDBAndFail(t, errors.New("wrong value from db"), database)
		}
	}

	if err := db.Close(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}

func Test_CustomBinariesRepo(t *testing.T) {
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
		BinaryRepositoryURL("https://repo.maven.apache.org/maven2").
		Port(9876).
		StartTimeout(10 * time.Second).
		Locale("C").
		Logger(nil))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=9876 user=gin password=wine dbname=beer sslmode=disable")
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

func Test_CustomBinariesLocation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().
		BinariesPath(tempDir))

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	// Delete cache to make sure unarchive doesn't happen again.
	cacheLocation, _ := database.cacheLocator()
	if err := os.RemoveAll(cacheLocation); err != nil {
		panic(err)
	}

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}

func Test_PrefetchedBinaries(t *testing.T) {
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

	database := NewDatabase(DefaultConfig().
		BinariesPath(binTempDir).
		RuntimePath(runtimeTempDir))

	// Download and unarchive postgres into the bindir.
	if err := database.remoteFetchStrategy(); err != nil {
		panic(err)
	}

	cacheLocation, _ := database.cacheLocator()
	if err := decompressTarXz(defaultTarReader, cacheLocation, binTempDir); err != nil {
		panic(err)
	}

	// Expect everything to work without cacheLocator and/or remoteFetch abilities.
	database.cacheLocator = func() (string, bool) {
		return "", false
	}
	database.remoteFetchStrategy = func() error {
		return errors.New("did not work")
	}

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	if err := database.Stop(); err != nil {
		shutdownDBAndFail(t, err, database)
	}
}

func Test_RunningInParallel(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "parallel_tests_path")
	if err != nil {
		panic(err)
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(2)

	runTestWithPortAndPath := func(port uint32, path string) {
		defer waitGroup.Done()

		database := NewDatabase(DefaultConfig().Port(port).RuntimePath(path))
		if err := database.Start(); err != nil {
			shutdownDBAndFail(t, err, database)
		}

		db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=%d user=postgres password=postgres dbname=postgres sslmode=disable", port))
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

	go runTestWithPortAndPath(8765, path.Join(tempPath, "1"))
	go runTestWithPortAndPath(8766, path.Join(tempPath, "2"))

	waitGroup.Wait()
}
