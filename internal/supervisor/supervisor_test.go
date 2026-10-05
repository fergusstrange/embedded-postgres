package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestLeaseOwner is a separate process so killing it cannot run Go defers.
func TestLeaseOwner(t *testing.T) {
	if os.Getenv("EP_LEASE_OWNER") != "1" {
		return
	}
	cmd, release, err := platform.Command(context.Background(), nil, os.Getenv("EP_SUPERVISOR"), "__supervise")
	if err != nil {
		panic(err)
	}
	pipe, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	release()
	var req Request
	if err = json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}
	if err = json.NewEncoder(pipe).Encode(req); err != nil {
		panic(err)
	}
	if os.Getenv("EP_CRASH_MODE") == "panic" {
		var trigger [1]byte
		_, _ = os.Stdin.Read(trigger[:])
		go func() { panic("intentional owner crash") }()
	}
	time.Sleep(24 * time.Hour)
}

func TestSupervisorParentDeath(t *testing.T) {
	bin, helper := os.Getenv("EP_TEST_BIN"), os.Getenv("EP_SUPERVISOR")
	if bin == "" || helper == "" {
		t.Skip("set EP_TEST_BIN and EP_SUPERVISOR for real process tests")
	}
	var id *platform.Identity
	if uid := os.Getenv("EP_TEST_UID"); uid != "" {
		u, e := strconv.ParseUint(uid, 10, 32)
		if e != nil {
			t.Fatal(e)
		}
		g, e := strconv.ParseUint(os.Getenv("EP_TEST_GID"), 10, 32)
		if e != nil {
			t.Fatal(e)
		}
		id = &platform.Identity{UID: uint32(u), GID: uint32(g)}
	}
	for _, how := range []string{"close", "kill", "panic"} {
		t.Run(how, func(t *testing.T) {
			work, err := os.MkdirTemp("", "ep-supervision-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(work) })
			if err = platform.Own(work, id); err != nil {
				t.Fatal(err)
			}
			data := filepath.Join(work, "data")
			log := filepath.Join(work, "postgres.log")
			if err = os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err = platform.Own(log, id); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			init, release, err := platform.Command(ctx, id, filepath.Join(bin, executable("initdb")), "-D", data, "-U", "postgres", "-A", "trust", "--locale=C", "-E", "UTF8")
			if err != nil {
				t.Fatal(err)
			}
			init.Dir = work
			out, err := init.CombinedOutput()
			release()
			if err != nil {
				t.Fatalf("initdb: %v\n%s", err, out)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := listener.Addr().(*net.TCPAddr).Port
			listener.Close()
			req := Request{Protocol: Protocol, Binary: filepath.Join(bin, executable("postgres")), DataDir: data, LogPath: log, Args: []string{"-D", data, "-h", "127.0.0.1", "-p", strconv.Itoa(port), "-c", "unix_socket_directories="}, ShutdownTimeout: 5 * time.Second}
			exe, args := helper, []string{"__supervise"}
			if how != "close" {
				exe, _ = os.Executable()
				args = []string{"-test.run=^TestLeaseOwner$"}
			}
			owner, release, err := platform.Command(ctx, id, exe, args...)
			if err != nil {
				t.Fatal(err)
			}
			owner.Dir = work
			owner.Env = append(os.Environ(), "EP_LEASE_OWNER=1", "EP_SUPERVISOR="+helper, "EP_CRASH_MODE="+how)
			input, err := owner.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := owner.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			owner.Stderr = os.Stderr
			if err = owner.Start(); err != nil {
				t.Fatal(err)
			}
			release()
			defer func() { input.Close(); _ = owner.Process.Kill() }()
			if err = json.NewEncoder(input).Encode(req); err != nil {
				t.Fatal(err)
			}
			var event Event
			if err = json.NewDecoder(output).Decode(&event); err != nil {
				t.Fatal(err)
			}
			if event.PID == 0 {
				t.Fatalf("no owned process: %+v", event)
			}
			t.Cleanup(func() {
				if _, e := os.Stat(filepath.Join(data, "postmaster.pid")); e != nil {
					return
				}
				p, e := os.FindProcess(event.PID)
				if e == nil {
					_ = platform.Kill(p)
				}
			})
			// Readiness must authenticate to the exact server, not just observe an open port.
			for {
				check, release, e := platform.Command(ctx, id, filepath.Join(bin, executable("psql")), "-X", "-w", "-h", "127.0.0.1", "-p", strconv.Itoa(port), "-U", "postgres", "-d", "postgres", "-Atc", "SELECT 1")
				if e != nil {
					t.Fatal(e)
				}
				check.Dir = work
				out, e = check.CombinedOutput()
				release()
				if e == nil {
					break
				}
				if ctx.Err() != nil {
					b, _ := os.ReadFile(log)
					t.Fatalf("readiness: %s %s", out, b)
				}
				time.Sleep(50 * time.Millisecond)
			}
			if how == "close" {
				input.Close()
			} else if how == "kill" {
				_ = owner.Process.Kill()
			} else {
				_, _ = input.Write([]byte("!"))
			}
			_ = owner.Wait()
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				_, e := os.Stat(filepath.Join(data, "postmaster.pid"))
				if os.IsNotExist(e) {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			logs, _ := os.ReadFile(log)
			t.Fatalf("PostgreSQL survived %s: %s", how, logs)
		})
	}
}

func TestRejectProtocol(t *testing.T) {
	inputR, inputW := io.Pipe()
	go func() { fmt.Fprintln(inputW, `{"protocol":999}`); inputW.Close() }()
	// Guard deliberately owns a Windows process Job; run protocol validation only
	// in the helper on Windows (the integration test exercises it there).
	if executable("x") == "x.exe" {
		inputR.Close()
		return
	}
	if err := Run(inputR, io.Discard); err == nil {
		t.Fatal("invalid protocol accepted")
	}
}
