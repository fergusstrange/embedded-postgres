package embeddedpostgres

import (
	"archive/tar"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xi2/xz"
)

func Test_decompressTarXz(t *testing.T) {
	defer verifyLeak(t)

	tempDir, err := ioutil.TempDir("", "temp_tar_test")
	if err != nil {
		panic(err)
	}

	archive, cleanUp := createTempXzArchive()
	defer cleanUp()

	err = decompressTarXz(defaultTarReader, archive, tempDir)

	assert.NoError(t, err)

	expectedExtractedFileLocation := filepath.Join(tempDir, "dir1", "dir2", "some_content")
	assert.FileExists(t, expectedExtractedFileLocation)

	fileContentBytes, err := ioutil.ReadFile(expectedExtractedFileLocation)
	assert.NoError(t, err)

	assert.Equal(t, "b33r is g00d", string(fileContentBytes))
}

func Test_decompressTarXz_ErrorWhenFileNotExists(t *testing.T) {
	defer verifyLeak(t)

	err := decompressTarXz(defaultTarReader, "/does-not-exist", "/also-fake")

	assert.EqualError(t, err, "unable to extract postgres archive /does-not-exist to /also-fake, if running parallel tests, configure RuntimePath to isolate testing directories")
}

func Test_decompressTarXz_ErrorWhenErrorDuringRead(t *testing.T) {
	defer verifyLeak(t)

	tempDir, err := ioutil.TempDir("", "temp_tar_test")
	if err != nil {
		panic(err)
	}

	archive, cleanUp := createTempXzArchive()
	defer cleanUp()

	err = decompressTarXz(func(reader *xz.Reader) (func() (*tar.Header, error), func() io.Reader) {
		return func() (*tar.Header, error) {
			return nil, errors.New("oh noes")
		}, nil
	}, archive, tempDir)

	assert.EqualError(t, err, "unable to extract postgres archive: oh noes")
}

func Test_decompressTarXz_ErrorWhenFailedToReadFileToCopy(t *testing.T) {
	defer verifyLeak(t)

	tempDir, err := ioutil.TempDir("", "temp_tar_test")
	if err != nil {
		panic(err)
	}

	archive, cleanUp := createTempXzArchive()
	defer cleanUp()

	blockingFile := filepath.Join(tempDir, "blocking")

	if err = ioutil.WriteFile(blockingFile, []byte("wazz"), 0000); err != nil {
		panic(err)
	}

	fileBlockingExtractTarReader := func(reader *xz.Reader) (func() (*tar.Header, error), func() io.Reader) {
		shouldReadFile := true

		return func() (*tar.Header, error) {
				if shouldReadFile {
					shouldReadFile = false

					return &tar.Header{
						Typeflag: tar.TypeReg,
						Name:     "blocking",
					}, nil
				}

				return nil, io.EOF
			}, func() io.Reader {
				open, _ := os.Open("file_not_exists")
				return open
			}
	}

	err = decompressTarXz(fileBlockingExtractTarReader, archive, tempDir)

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}

func Test_decompressTarXz_ErrorWhenFileToCopyToNotExists(t *testing.T) {
	defer verifyLeak(t)

	tempDir, err := ioutil.TempDir("", "temp_tar_test")
	if err != nil {
		panic(err)
	}

	archive, cleanUp := createTempXzArchive()
	defer cleanUp()

	fileBlockingExtractTarReader := func(reader *xz.Reader) (func() (*tar.Header, error), func() io.Reader) {
		shouldReadFile := true

		return func() (*tar.Header, error) {
				if shouldReadFile {
					shouldReadFile = false

					return &tar.Header{
						Typeflag: tar.TypeReg,
						Name:     "some_dir/wazz/dazz/fazz",
					}, nil
				}

				return nil, io.EOF
			}, func() io.Reader {
				open, _ := os.Open("file_not_exists")
				return open
			}
	}

	err = decompressTarXz(fileBlockingExtractTarReader, archive, tempDir)

	assert.Regexp(t, "^unable to extract postgres archive:.+$", err)
}
