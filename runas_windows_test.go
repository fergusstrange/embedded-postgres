//go:build windows
// +build windows

package embeddedpostgres

import (
	"io/ioutil"
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultInitDatabase_RunAsNotSupported(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().RuntimePath(tempDir).RunAsUser("user"))
	err = database.Start()
	assert.EqualError(t, err, "runAsUser config parameter not supported on windows")
}
