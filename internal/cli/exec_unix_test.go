//go:build !windows

package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInterruptedCommandChild(t *testing.T) {
	mode := os.Getenv("EP_INTERRUPTION_TEST")
	if mode == "" {
		return
	}
	if mode == "self-terminate" {
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
		time.Sleep(time.Minute)
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	fmt.Println("child-ready")
	if mode == "ignore" {
		time.Sleep(time.Minute)
		return
	}
	<-signals
	// This is a test framework's afterAll hook: the database must remain usable.
	time.Sleep(100 * time.Millisecond)
	psql := exec.Command(filepath.Join(os.Getenv("EP_TEST_BIN"), "psql"), "-X", "-w", "-Atc", "SELECT 1")
	if output, err := psql.CombinedOutput(); err != nil {
		t.Fatalf("database unavailable during teardown: %v %s", err, output)
	}
	fmt.Println("teardown-complete")
}

func TestCLIExecGracefulInterruption(t *testing.T) {
	bin, helper := cliFixture(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, helper, "exec", "--binaries", bin, "--", os.Args[0], "-test.run=^TestInterruptedCommandChild$")
			cmd.Env = append(os.Environ(), "EP_INTERRUPTION_TEST=graceful")
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			reader := bufio.NewReader(out)
			if line, err := reader.ReadString('\n'); err != nil || line != "child-ready\n" {
				t.Fatalf("child readiness: %q %v %s", line, err, &stderr)
			}
			if err = cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			tail := new(strings.Builder)
			_, _ = reader.WriteTo(tail)
			err = cmd.Wait()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 128+int(sig) || !strings.Contains(tail.String(), "teardown-complete") {
				t.Fatalf("interruption lost teardown or exit code: %v\n%s\n%s", err, tail, &stderr)
			}
		})
	}
}

func TestCommandSignalExitAndEscalation(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestInterruptedCommandChild$")
	cmd.Env = append(os.Environ(), "EP_INTERRUPTION_TEST=self-terminate")
	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || commandExitCode(exit) != 143 {
		t.Fatal("signal exit not preserved", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInterruptedCommandChild$")
	cmd.Env = append(os.Environ(), "EP_INTERRUPTION_TEST=ignore")
	cmd.Cancel = func() error { return interruptCommand(cmd.Process, ctx) }
	cmd.WaitDelay = 50 * time.Millisecond
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if _, err = bufio.NewReader(output).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err = cmd.Wait(); err == nil {
		t.Fatal("ignored signal did not escalate")
	}
	if interruptionCode(ctx) != 143 {
		t.Fatal("plain cancellation code")
	}
}
