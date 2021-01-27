package embeddedpostgres

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholt/archiver"
)

func createTempXzArchive() (string, func()) {
	tempDir, err := ioutil.TempDir("", "remote_fetch_test")
	if err != nil {
		panic(err)
	}

	tempFile, err := ioutil.TempFile(tempDir, "remote_fetch_test")
	if err != nil {
		panic(err)
	}

	tarFile := filepath.Join(tempDir, "remote_fetch_test.txz")
	if err := archiver.NewTarXz().Archive([]string{tempFile.Name()}, tarFile); err != nil {
		panic(err)
	}

	return tarFile, func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}
}

func createTempZipArchive() (string, func()) {
	tempDir, err := ioutil.TempDir("", "remote_fetch_test")
	if err != nil {
		panic(err)
	}

	tempFile, err := ioutil.TempFile(tempDir, "remote_fetch_test")
	if err != nil {
		panic(err)
	}

	tarFile := filepath.Join(tempDir, "remote_fetch_test.txz")
	if err := archiver.NewTarXz().Archive([]string{tempFile.Name()}, tarFile); err != nil {
		panic(err)
	}

	jarFile := filepath.Join(tempDir, "remote_fetch_test.zip")
	if err := archiver.NewZip().Archive([]string{tempFile.Name(), tarFile}, jarFile); err != nil {
		panic(err)
	}

	return jarFile, func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}
}

func shutdownDBAndFail(t *testing.T, err error, db *EmbeddedPostgres) {
	if db.started {
		if stopErr := db.Stop(); stopErr != nil {
			t.Errorf("Failed to shutdown server with error %s", stopErr)
		}
	}
	t.Errorf("Failed for version %s with error %s", db.config.version, err)
}

func testVersionStrategy() VersionStrategy {
	return func() (string, string, PostgresVersion) {
		return "darwin", "amd64", "1.2.3"
	}
}

func testCacheLocator() CacheLocator {
	return func() (s string, b bool) {
		return "", false
	}
}
