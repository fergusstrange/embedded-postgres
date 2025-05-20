//go:build !windows
// +build !windows

package embeddedpostgres

import (
	"os/exec"
	"syscall"
)

func applyPlatformSpecificOptions(cmd *exec.Cmd, config Config) {
	if config.ownProcessGroup {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Setpgid = true
	}
}
