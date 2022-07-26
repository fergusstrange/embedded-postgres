//go:build !windows
// +build !windows

package embeddedpostgres

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultInitDatabase_RunAsUnknownUser(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().RuntimePath(tempDir).RunAsUser("+"))
	err = database.Start()
	assert.EqualError(t, err, "unable to lookup run-as user '+': user: unknown user +")
}

func Test_defaultInitDatabase_RunAsSameUser(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "prepare_database_test")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	currentUser, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	// Same user
	username := currentUser.Username

	database := NewDatabase(DefaultConfig().RuntimePath(tempDir).RunAsUser(username))
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()
}

func Test_RunAsUnknownUser(t *testing.T) {
	process := exec.Command("bash", "-c", "whoami")
	missingUser := "+"
	err := setRunAs(process, "+")
	assert.EqualError(t, err, fmt.Sprintf("unable to lookup run-as user '%[1]s': user: unknown user %[1]s", missingUser))
}

func Test_ChownUnknownUser(t *testing.T) {
	missingUser := "+"
	file := "file"
	err := chown(file, missingUser)
	assert.EqualError(t, err, fmt.Sprintf("unable to lookup run-as user '%[1]s': user: unknown user %[1]s", missingUser))
}

func Test_ChownMissingFile(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	username := currentUser.Username
	missingFile := "+"
	err = chown("+", username)
	assert.EqualError(t, err, fmt.Sprintf("unable to chown '%[2]s' file with '%[1]s': chown %[2]s: no such file or directory", username, missingFile))
}
