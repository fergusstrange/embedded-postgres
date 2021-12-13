//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package embeddedpostgres

import (
	"database/sql"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RunAsUser(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "embedded_postgres_test")
	require.NoError(t, err)

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	current, err := user.Current()
	require.NoError(t, err)
	uid, err := strconv.ParseInt(current.Uid, 10, 64)
	require.NoError(t, err)
	gid, err := strconv.ParseInt(current.Gid, 10, 64)
	require.NoError(t, err)

	t.Logf("Running as %d/%d", uid, gid)

	database := NewDatabase(DefaultConfig().
		ProcAttr(
			&syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid:         uint32(uid),
					Gid:         uint32(gid),
					NoSetGroups: true,
				},
			},
		).
		Username("gin").
		Password("wine").
		Database("beer").
		Version(V12).
		RuntimePath(tempDir).
		Port(9876).
		StartTimeout(10 * time.Second).
		Locale("C").
		Logger(nil),
	)

	if err := database.Start(); err != nil {
		shutdownDBAndFail(t, err, database)
	}

	db, err := sql.Open("postgres", "host=localhost port=9876 user=gin password=wine dbname=beer sslmode=disable")
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

func Test_RunAsNonexistentUser(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "embedded_postgres_test")
	require.NoError(t, err)

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	database := NewDatabase(DefaultConfig().
		ProcAttr(
			&syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(100000),
					Gid: uint32(94959495),
				},
			},
		).
		Username("gin").
		Password("wine").
		Database("beer").
		Version(V12).
		RuntimePath(tempDir).
		Port(9876).
		StartTimeout(10 * time.Second).
		Locale("C").
		Logger(nil),
	)

	err = database.Start()
	assert.Error(t, err)
}
