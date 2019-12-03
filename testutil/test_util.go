package testutil

import (
	"github.com/mholt/archiver"
	"io/ioutil"
	"os"
	"path/filepath"
)

func CreateTempXzArchive() (string, func()) {
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

func CreateTempZipArchive() (string, func()) {
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
