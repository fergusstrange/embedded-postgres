//go:build linux || darwin || freebsd

package platform

import (
	"errors"
	"fmt"
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
	if id.UID == 0 || id.UID == ^uint32(0) || id.GID == ^uint32(0) {
		return errors.New("RunAs needs a nonzero UID and cannot use the Unix unchanged-ID sentinel")
	}
	if os.Geteuid() != 0 && (int(id.UID) != os.Geteuid() || int(id.GID) != os.Getegid()) {
		return errors.New("changing OS identity requires root")
	}
	return nil
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
	if err := os.Chown(path, int(id.UID), int(id.GID)); err != nil {
		return fmt.Errorf("assign %s to %d:%d: %w", path, id.UID, id.GID, err)
	}
	return nil
}

func Guard() error { return nil }
