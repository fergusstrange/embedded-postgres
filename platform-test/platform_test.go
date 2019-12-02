package platform_test

import (
	"database/sql"
	"fmt"
	"github.com/fergusstrange/embedded-postgres"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func Test_AllMajorVersions(t *testing.T) {
	allVersions := []embeddedpostgres.PostgresVersion{embeddedpostgres.V12_1_0, embeddedpostgres.V11_6_0, embeddedpostgres.V10_11_0, embeddedpostgres.V9_6_16}
	tempExtractLocation, err := ioutil.TempDir("", "embedded_postgres_go_tests")
	if err != nil {
		t.Fatal(err)
	}

	for testNumber, version := range allVersions {
		t.Run(fmt.Sprintf("MajorVersion_%d", testNumber), func(t *testing.T) {
			port := uint32(5555 + testNumber)
			database := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
				Version(version).
				Port(port).
				RuntimePath(filepath.Join(tempExtractLocation, strconv.Itoa(testNumber))))

			if err := database.Start(); err != nil {
				shutdownDBAndFail(t, err, database, version)
			}

			db, err := connect(port)
			if err != nil {
				shutdownDBAndFail(t, err, database, version)
			}

			rows, err := db.Query("SELECT 1")
			if err != nil {
				shutdownDBAndFail(t, err, database, version)
			}
			if err := rows.Close(); err != nil {
				shutdownDBAndFail(t, err, database, version)
			}

			if err := db.Close(); err != nil {
				shutdownDBAndFail(t, err, database, version)
			}

			if err := database.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := os.RemoveAll(tempExtractLocation); err != nil {
		t.Fatal(err)
	}
}

func shutdownDBAndFail(t *testing.T, err error, db *embeddedpostgres.EmbeddedPostgres, version embeddedpostgres.PostgresVersion) {
	if err := db.Stop(); err != nil {
		t.Fatalf("Failed for version %s with error %s", version, err)
	}
	t.Fatalf("Failed for version %s with error %s", version, err)
}

func connect(port uint32) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=%d user=postgres password=postgres dbname=postgres sslmode=disable", port))
	return db, err
}
