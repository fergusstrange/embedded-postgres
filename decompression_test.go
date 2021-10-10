package embeddedpostgres

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_unTar(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "temp_tar_test")
	if err != nil {
		panic(err)
	}

	archive, cleanUp := createTempXzArchive()
	defer cleanUp()

	err = unTar(archive, tempDir)

	assert.NoError(t, err)

	expectedExtractedFileLocation := filepath.Join(tempDir, "dir1", "dir2", "some_content")
	assert.FileExists(t, expectedExtractedFileLocation)

	fileContentBytes, err := ioutil.ReadFile(expectedExtractedFileLocation)
	assert.NoError(t, err)

	assert.Equal(t, "b33r is g00d", string(fileContentBytes))
}
