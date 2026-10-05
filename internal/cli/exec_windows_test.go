//go:build windows

package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestWindowsExecChild(t *testing.T) {
	if os.Getenv("EP_WINDOWS_EXEC_CHILD") != "1" {
		return
	}
	fmt.Println("child-ready")
	time.Sleep(time.Minute)
}

func TestWindowsExecCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsExecChild$")
	cmd.Env = append(os.Environ(), "EP_WINDOWS_EXEC_CHILD=1")
	cmd.Cancel = func() error { return interruptCommand(cmd.Process, ctx) }
	cmd.WaitDelay = time.Second
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "child-ready\n" {
		t.Fatal("child did not start", line, err)
	}
	cancel()
	var exit *exec.ExitError
	if err = cmd.Wait(); !errors.As(err, &exit) || commandExitCode(exit) == 0 || interruptionCode(ctx) != 143 {
		t.Fatal("Windows cancellation failed", err)
	}
}
