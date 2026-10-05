//go:build windows

package cli

import (
	"context"
	"os"
	"os/exec"
)

// Windows has no portable per-process SIGINT/SIGTERM forwarding API.
func interruptCommand(p *os.Process, _ context.Context) error { return p.Kill() }
func commandExitCode(err *exec.ExitError) int                 { return max(1, err.ExitCode()) }
