package supervisor

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func helperFixture(t *testing.T) string {
	t.Helper()
	path := os.Getenv("EP_SUPERVISOR")
	if path == "" {
		t.Skip("set EP_SUPERVISOR")
	}
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("non-root supervisor request tests")
	}
	return path
}
func TestRejectedSubprocessRequests(t *testing.T) {
	helper := helperFixture(t)
	for _, tc := range []struct{ command, input string }{{"__supervise", "invalid"}, {"__command", "invalid"}, {"__command", `{"protocol":999}`}, {"__supervise", `{"protocol":1,"shutdown_timeout":0}`}, {"__supervise", `{"protocol":1,"shutdown_timeout":1000000000,"log_path":"missing"}`}, {"__command", `{"protocol":1,"binary":"missing"}`}} {
		cmd := exec.CommandContext(t.Context(), helper, tc.command)
		cmd.Stdin = strings.NewReader(tc.input)
		if e := cmd.Run(); e == nil {
			t.Fatal("invalid request accepted", tc.command, tc.input)
		}
	}
}
func TestCommandChild(t *testing.T) {
	path := os.Getenv("EP_COMMAND_CHILD_MARKER")
	if path == "" {
		return
	}
	if e := os.WriteFile(path, []byte("started"), 0600); e != nil {
		os.Exit(3)
	}
	time.Sleep(time.Minute)
}
func TestSupervisorForcedShutdown(t *testing.T) {
	helper := helperFixture(t)
	for _, mode := range []string{"__command", "__supervise"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "child")
			log := filepath.Join(dir, "log")
			os.WriteFile(log, nil, 0600)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, helper, mode)
			cmd.Env = append(os.Environ(), "EP_COMMAND_CHILD_MARKER="+marker)
			input, e := cmd.StdinPipe()
			if e != nil {
				t.Fatal(e)
			}
			output, e := cmd.StdoutPipe()
			if e != nil {
				t.Fatal(e)
			}
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() {
				input.Close()
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			req := Request{Protocol: Protocol, Binary: os.Args[0], Args: []string{"-test.run=^TestCommandChild$"}, DataDir: dir, LogPath: log, ShutdownTimeout: time.Second}
			if e = json.NewEncoder(input).Encode(req); e != nil {
				t.Fatal(e)
			}
			if mode == "__supervise" {
				var event Event
				if e = json.NewDecoder(output).Decode(&event); e != nil || event.PID == 0 {
					t.Fatal(event, e)
				}
			}
			for {
				if _, e = os.Stat(marker); e == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("child did not start")
				case <-time.After(10 * time.Millisecond):
				}
			}
			input.Close()
			_, _ = io.Copy(io.Discard, output)
			e = cmd.Wait()
			if ctx.Err() != nil {
				t.Fatal("supervisor did not reap canceled child")
			}
			if mode == "__supervise" && e != nil {
				t.Fatal("fallback shutdown failed", e)
			}
		})
	}
}
