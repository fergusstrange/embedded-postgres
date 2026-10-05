//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS: do not inherit the launcher's console.
		HideWindow: true}
}
