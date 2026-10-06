//go:build linux || darwin || freebsd

package platform

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"syscall"
)

func ValidateIdentity(id *Identity) error {
	if id == nil {
		if os.Geteuid() == 0 {
			return errors.New("PostgreSQL cannot run as root: select an existing non-root OS identity with RunAs (CLI: --user UID:GID)")
		}
		return nil
	}
	uid, gid, err := nativeIdentity(*id)
	if err != nil {
		return err
	}
	if os.Geteuid() == 0 && gid == 0 {
		return errors.New("RunAs needs a nonzero GID for a root caller: set the account's primary group explicitly")
	}
	if os.Geteuid() != 0 && (uid != os.Geteuid() || gid != os.Getegid()) {
		return errors.New("changing OS identity requires root")
	}
	return nil
}

// Go's ownership APIs use int, which is narrower than a Unix ID on 32-bit hosts.
func nativeIdentity(id Identity) (int, int, error) {
	uid, gid := uint64(id.UID), uint64(id.GID)
	if uid == 0 || uid == math.MaxUint32 || gid == math.MaxUint32 {
		return 0, 0, errors.New("RunAs needs a nonzero UID and cannot use the Unix unchanged-ID sentinel")
	}
	if uid > math.MaxInt || gid > math.MaxInt {
		return 0, 0, errors.New("RunAs UID and GID must fit the host's int range")
	}
	return int(uid), int(gid), nil
}

func configure(cmd *exec.Cmd, id *Identity) (func(), error) {
	if err := ValidateIdentity(id); err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if id != nil {
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: id.UID, Gid: id.GID, NoSetGroups: os.Geteuid() != 0}
	}
	return func() {}, nil
}

// Kill terminates only a process group created by Command.
func Kill(p *os.Process) error {
	if p == nil {
		return nil
	}
	err := syscall.Kill(-p.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func Own(path string, id *Identity) error {
	if id == nil {
		return nil
	}
	uid, gid, err := nativeIdentity(*id)
	if err != nil {
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("assign %s to %d:%d: %w", path, id.UID, id.GID, err)
	}
	return nil
}

func Guard() error { return nil }
