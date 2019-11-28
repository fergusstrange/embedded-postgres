package embeddedpostgres

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func Test_AllMajorVersions(t *testing.T) {
	allVersions := []PostgresVersion{V12_1_0, V11_6_0, V10_11_0, V9_6_16}
	tempExtractLocation, err := ioutil.TempDir("", "embedded_postgres_go_tests")
	if err != nil {
		t.Fatal(err)
	}

	for testNumber, version := range allVersions {
		port := uint32(5555 + testNumber)
		database := NewDatabase(DefaultConfig().
			Version(version).
			Port(port).
			RuntimePath(filepath.Join(tempExtractLocation, strconv.Itoa(testNumber))))

		if err := database.Start(); err != nil {
			shutdownDBAndFail(t, err, database)
		}

		db, err := connect(port)
		if err != nil {
			shutdownDBAndFail(t, err, database)
		}

		rows, err := db.Query("SELECT 1")
		if err != nil {
			shutdownDBAndFail(t, err, database)
		}
		if err := rows.Close(); err != nil {
			shutdownDBAndFail(t, err, database)
		}

		if err := db.Close(); err != nil {
			shutdownDBAndFail(t, err, database)
		}

		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(tempExtractLocation); err != nil {
		t.Fatal(err)
	}
}

func Test_CustomDatabaseName(t *testing.T) {
	database := NewDatabase(DefaultConfig().
		Username("gin").
		Password("wine").
		Database("beer").
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

func shutdownDBAndFail(t *testing.T, err error, db *EmbeddedPostgres) {
	if err := db.Stop(); err != nil {
		t.Fatalf("Failed for version %s with error %s", db.config.version, err)
	}
	t.Fatalf("Failed for version %s with error %s", db.config.version, err)
}

func connect(port uint32) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=%d user=postgres password=postgres dbname=postgres sslmode=disable", port))
	return db, err
}
