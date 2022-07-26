//go:build !windows
// +build !windows

package embeddedpostgres

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func lookupUser(runAsUser string) (uint32, uint32, error) {
	u, err := user.Lookup(runAsUser)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to lookup run-as user '%s': %w", runAsUser, err)
	}

	uid, err := strconv.ParseInt(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to get uid of run-as user '%s': %w", runAsUser, err)
	}

	gid, err := strconv.ParseInt(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to get gid of run-as user '%s': %w", runAsUser, err)
	}

	return uint32(uid), uint32(gid), nil
}

func setRunAs(process *exec.Cmd, runAsUser string) error {
	uid, gid, err := lookupUser(runAsUser)
	if err != nil {
		return err
	}

	process.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: gid, NoSetGroups: true},
	}

	return nil
}

func chown(file string, runAsUser string) error {
	uid, gid, err := lookupUser(runAsUser)
	if err != nil {
		return err
	}

	err = os.Chown(file, int(uid), int(gid))
	if err != nil {
		return fmt.Errorf("unable to chown '%s' file with '%s': %w", file, runAsUser, err)
	}

	return nil
}
