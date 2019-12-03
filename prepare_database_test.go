package embeddedpostgres

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func Test_defaultInitDatabase_ErrorWhenCannotCreatePasswordFile(t *testing.T) {
	err := defaultInitDatabase("path_not_exists", "Tom", "Beer")

	assert.EqualError(t, err, "unable to write password file to path_not_exists/pwfile")
}

func Test_defaultInitDatabase_ErrorWhenCannotStartInitDBProcess(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	err = defaultInitDatabase(tempDir, "Tom", "Beer")

	assert.EqualError(t, err, fmt.Sprintf("unable to init database using: %s/bin/initdb -A password -U Tom -D %s/data --pwfile=%s/pwfile",
		tempDir,
		tempDir,
		tempDir))
	assert.FileExists(t, filepath.Join(tempDir, "pwfile"))
}
