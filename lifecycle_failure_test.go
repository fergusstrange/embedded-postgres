package embeddedpostgres

import (
	"context"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLifecycleFilesystemFailures(t *testing.T) {
	c := integrationConfig(t)
	for _, mode := range []string{"data-file", "data-parent-file", "initdb-encoding", "password-path", "removed-data"} {
		t.Run(mode, func(t *testing.T) {
			cfg := c
			parent, e := os.MkdirTemp("", "ep-failure-")
			if e != nil {
				t.Fatal(e)
			}
			defer os.RemoveAll(parent)
			if c.identity != nil {
				os.Chmod(parent, 0755)
			}
			switch mode {
			case "data-file", "data-parent-file":
				file := filepath.Join(parent, "file")
				os.WriteFile(file, nil, 0600)
				if mode == "data-parent-file" {
					file = filepath.Join(file, "data")
				}
				cfg = cfg.DataPath(file)
			case "initdb-encoding":
				cfg = cfg.Encoding("this_encoding_does_not_exist")
			case "password-path":
				cfg = cfg.Hooks(Hooks{BeforeStart: []Hook{func(_ context.Context, i InstanceInfo) (func(context.Context) error, error) {
					return nil, os.Mkdir(filepath.Join(i.WorkDir, "pgpass"), 0700)
				}}})
			case "removed-data":
				cfg = cfg.Hooks(Hooks{BeforeStart: []Hook{func(_ context.Context, i InstanceInfo) (func(context.Context) error, error) {
					return nil, os.RemoveAll(i.DataDir)
				}}})
			}
			pg := NewDatabase(cfg)
			if e = pg.Start(); e == nil {
				pg.Close()
				t.Fatal("expected startup failure")
			}
			if pg.active != nil {
				t.Fatal("failed session not cleaned")
			}
		})
	}
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		parent := t.TempDir()
		os.Chmod(parent, 0000)
		defer os.Chmod(parent, 0700)
		if e := NewDatabase(c.RuntimePath(parent)).Start(); e == nil {
			t.Fatal("unwritable workspace accepted")
		}
	}
}
func TestAsyncCleanupPanicReported(t *testing.T) {
	c := integrationConfig(t).Hooks(Hooks{Ready: []Hook{func(context.Context, InstanceInfo) (func(context.Context) error, error) {
		return func(context.Context) error { panic("cleanup failed") }, nil
	}}})
	pg := NewDatabase(c)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if e := pg.StartContext(ctx); e != nil {
		t.Fatal(e)
	}
	done := pg.Done()
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("panic blocked async cleanup")
	}
	if pg.Err() == nil {
		t.Fatal("async cleanup error lost")
	}
	if _, e := os.Stat(pg.Info().WorkDir); !os.IsNotExist(e) {
		t.Fatal("panic retained workspace", e)
	}
}
func TestStalledProcess(t *testing.T) {
	if os.Getenv("EP_STALLED_CHILD") == "1" {
		time.Sleep(time.Minute)
	}
}
func TestUnresponsiveSupervisorFallback(t *testing.T) {
	c := DefaultConfig().StopTimeout(time.Millisecond)
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		c = integrationConfig(t).StopTimeout(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	child, release, e := platform.Command(ctx, c.identity, os.Args[0], "-test.run=^TestStalledProcess$")
	if e != nil {
		t.Fatal(e)
	}
	child.Env = append(child.Env, "EP_STALLED_CHILD=1")
	if e = child.Start(); e != nil {
		release()
		t.Fatal(e)
	}
	release()
	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()
	childReaped := false
	defer func() {
		if !childReaped {
			_ = platform.Kill(child.Process)
			<-childDone
		}
	}()
	owner, release, e := platform.Command(ctx, c.identity, os.Args[0], "-test.run=^TestStalledProcess$")
	if e != nil {
		t.Fatal(e)
	}
	owner.Env = append(owner.Env, "EP_STALLED_CHILD=1")
	lease, e := owner.StdinPipe()
	if e != nil {
		release()
		t.Fatal(e)
	}
	if e = owner.Start(); e != nil {
		release()
		t.Fatal(e)
	}
	release()
	s := &session{cmd: owner, lease: lease, postgresPID: child.Process.Pid, exited: make(chan struct{})}
	go func() { s.waitErr = owner.Wait(); close(s.exited) }()
	pg := NewDatabase(c)
	if e = pg.stopProcess(s); e == nil {
		t.Fatal("forced termination error lost")
	}
	select {
	case <-childDone:
		childReaped = true
	case <-ctx.Done():
		t.Fatal("owned child survived fallback")
	}
	if s.lease != nil {
		t.Fatal("owner lease remains")
	}
	if ctx.Err() != nil {
		t.Fatal("fallback exceeded deadline")
	}
}
