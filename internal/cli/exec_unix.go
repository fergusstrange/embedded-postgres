//go:build !windows

package cli

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func interruptCommand(p *os.Process, ctx context.Context) error {
	return p.Signal(interruptionSignal(ctx))
}
func commandExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return max(1, err.ExitCode())
}
