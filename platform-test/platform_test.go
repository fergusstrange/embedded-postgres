package platform_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func Test_AllMajorVersions(t *testing.T) {
	allVersions := []embeddedpostgres.PostgresVersion{
		embeddedpostgres.V9,
		embeddedpostgres.V10,
		embeddedpostgres.V11,
		embeddedpostgres.V12,
		embeddedpostgres.V13,
	}

	tempExtractLocation, err := ioutil.TempDir("", "embedded_postgres_go_tests")
	if err != nil {
		t.Fatal(err)
	}

	for i, v := range allVersions {
		testNumber := i
		version := v
		t.Run(fmt.Sprintf("MajorVersion_%s", version), func(t *testing.T) {
			port := uint32(5555 + testNumber)
			runtimePath := filepath.Join(tempExtractLocation, strconv.Itoa(testNumber))
			database := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
				Version(version).
				Port(port).
				RuntimePath(runtimePath))

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

			if err := checkPgVersionFile(filepath.Join(runtimePath, "data"), version); err != nil {
				t.Fatal(err)
			}
		})
	}

	if err := purgeTestDataOrWait(tempExtractLocation, 0); err != nil {
		t.Fatal(err)
	}
}

func purgeTestDataOrWait(tempExtractLocation string, attempts int) error {
	if err := os.RemoveAll(tempExtractLocation); err != nil {
		if attempts < 30 {
			log.Println("waiting for tests to clean up...")
			time.Sleep(1 * time.Second)

			return purgeTestDataOrWait(tempExtractLocation, attempts+1)
		}

		return err
	}

	return nil
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

func checkPgVersionFile(dataDir string, version embeddedpostgres.PostgresVersion) error {
	pgVersion := filepath.Join(dataDir, "PG_VERSION")

	d, err := ioutil.ReadFile(pgVersion)
	if err != nil {
		return fmt.Errorf("could not read file %v", pgVersion)
	}

	v := strings.TrimSuffix(string(d), "\n")

	if strings.HasPrefix(string(version), v) {
		return nil
	}

	return fmt.Errorf("version missmatch in PG_VERSION: %v <> %v", string(version), v)
}
