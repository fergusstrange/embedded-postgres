package embeddedpostgres

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConfigurationValidation(t *testing.T) {
	for _, c := range []Config{DefaultConfig().Port(65536), DefaultConfig().StartTimeout(0), DefaultConfig().StopTimeout(2 * time.Minute), DefaultConfig().Version("bad"), DefaultConfig().Version("18.x.0"), DefaultConfig().Database(""), DefaultConfig().Username("bad\nuser"), DefaultConfig().Password(""), DefaultConfig().Locale(""), DefaultConfig().Encoding(""), DefaultConfig().StartParameters(map[string]string{"PORT": "1234"}), DefaultConfig().StartParameters(map[string]string{"DATA_DIRECTORY": "other"}), DefaultConfig().StartParameters(map[string]string{"": "x"}), DefaultConfig().StartParameters(map[string]string{"custom": "\x00"}), DefaultConfig().Provider(LocalProvider(".")).BinaryRepositoryURL("mirror")} {
		if validateConfig(c) == nil {
			t.Errorf("invalid config accepted: %#v", c)
		}
	}
	c := DefaultConfig().CachePath("cache").RuntimePath("parent").DataPath("data").BinariesPath("distribution").Locale("C").Encoding("UTF8").Logger(&bytes.Buffer{}).Environment("EXTRA=value").Hooks(Hooks{BeforeStart: []Hook{nil}})
	copy := NewDatabase(c)
	c.environment[0] = "changed"
	c.hooks.BeforeStart[0] = func(context.Context, InstanceInfo) (func(context.Context) error, error) { return nil, nil }
	if copy.config.environment[0] != "EXTRA=value" || copy.config.hooks.BeforeStart[0] != nil {
		t.Fatal("config aliases caller collections")
	}
	if c.storage != (Storage{"cache", "parent", "data"}) {
		t.Fatal("legacy path mapping")
	}
	if !strings.Contains(c.UnixSocket("/tmp/socket").GetConnectionURL(), "host=%2Ftmp%2Fsocket") {
		t.Fatal("socket URL missing")
	}
}
func TestStartupProviderFailures(t *testing.T) {
	for _, mode := range []string{"error", "nil", "missing", "panic", "bad-work", "work-file", "missing-helper"} {
		t.Run(mode, func(t *testing.T) {
			released := false
			provider := ProviderFunc(func(context.Context, BinaryRequest) (*Installation, error) {
				switch mode {
				case "error":
					return nil, errors.New("unavailable")
				case "nil":
					return nil, nil
				case "panic":
					panic("provider panic")
				}
				return &Installation{Dir: filepath.Join(t.TempDir(), "missing"), Release: func() error { released = true; return nil }}, nil
			})
			c := DefaultConfig().Provider(provider)
			if runtime.GOOS != "windows" && os.Geteuid() == 0 {
				c = integrationConfig(t).BinariesPath("").Provider(provider)
			}
			work := t.TempDir()
			c = c.RuntimePath(work)
			if mode == "work-file" || mode == "bad-work" {
				file := filepath.Join(work, "file")
				os.WriteFile(file, nil, 0600)
				if mode == "bad-work" {
					file = filepath.Join(file, "child")
				}
				c = c.RuntimePath(file)
			}
			if mode == "missing-helper" {
				c = integrationConfig(t).RuntimePath(work).Supervisor(filepath.Join(work, "missing"))
			}
			pg := NewDatabase(c)
			func() {
				defer func() {
					if p := recover(); p != nil && mode != "panic" {
						t.Error(p)
					}
				}()
				if e := pg.Start(); e == nil {
					pg.Close()
					t.Error("expected startup failure")
				}
			}()
			if pg.active != nil {
				t.Fatal("failed session remains active")
			}
			if mode == "missing" && !released {
				t.Fatal("provider lease leaked")
			}
			if e := pg.Close(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPersistentVersionAndLock(t *testing.T) {
	c := integrationConfig(t)
	parent, err := os.MkdirTemp("", "ep-lock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(parent) })
	if c.identity != nil {
		os.Chmod(parent, 0755)
	}
	data := filepath.Join(parent, "data")
	pg := NewDatabase(c.DataPath(data))
	if e := pg.Start(); e != nil {
		t.Fatal(e)
	}
	defer pg.Close()
	if !errors.Is(pg.Start(), ErrServerAlreadyStarted) {
		t.Fatal("duplicate start accepted")
	}
	other := NewDatabase(c.DataPath(data).StartTimeout(150 * time.Millisecond))
	if e := other.Start(); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("data lock bypassed", e)
	}
	if e := pg.Close(); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(data, "PG_VERSION"), []byte("1\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := pg.Start(); !errors.Is(e, ErrDataVersion) {
		t.Fatal("wrong-major cluster accepted", e)
	}
}
func TestRestartWatcherIsolationAndStopDeadline(t *testing.T) {
	pg := NewDatabase(integrationConfig(t))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if e := pg.StartContext(ctx); e != nil {
		t.Fatal(e)
	}
	old := pg.active
	if e := pg.Stop(); e != nil {
		t.Fatal(e)
	}
	if e := pg.Start(); e != nil {
		t.Fatal(e)
	}
	defer pg.Close()
	cancel()
	if e := pg.closeSession(old); e != nil {
		t.Fatal(e)
	}
	select {
	case <-pg.Done():
		t.Fatal("old watcher closed new server")
	default:
	}
	if pg.GetConnectionURL() != pg.ConnectionURL() || pg.Info().Port != pg.GetPort() {
		t.Fatal("endpoint mismatch")
	}
	stop, cancelStop := context.WithCancel(t.Context())
	cancelStop()
	e := pg.StopContext(stop)
	if e != nil && !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	select {
	case <-pg.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("canceled StopContext abandoned cleanup")
	}
}
func TestHookCleanupOrderingAndPanic(t *testing.T) {
	c := integrationConfig(t)
	order := []int{}
	var work string
	before := func(_ context.Context, i InstanceInfo) (func(context.Context) error, error) {
		work = i.WorkDir
		return func(context.Context) error { order = append(order, 1); return errors.New("cleanup failed") }, nil
	}
	ready := func(context.Context, InstanceInfo) (func(context.Context) error, error) {
		return func(context.Context) error { order = append(order, 2); panic("cleanup panic") }, nil
	}
	pg := NewDatabase(c.Hooks(Hooks{BeforeStart: []Hook{before}, Ready: []Hook{ready}}))
	if e := pg.Start(); e != nil {
		t.Fatal(e)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("cleanup panic swallowed")
			}
		}()
		_ = pg.Close()
	}()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatal(order)
	}
	if _, e := os.Stat(work); !os.IsNotExist(e) {
		t.Fatal("cleanup panic retained workspace", e)
	}
	if e := pg.Close(); e != nil {
		t.Fatal(e)
	}
}
func TestBeforeStartFailureAndServerFailure(t *testing.T) {
	c := integrationConfig(t)
	cleaned := false
	hook := func(context.Context, InstanceInfo) (func(context.Context) error, error) {
		return func(context.Context) error { cleaned = true; return nil }, errors.New("before-start failed")
	}
	pg := NewDatabase(c.Hooks(Hooks{BeforeStart: []Hook{hook}}))
	if e := pg.Start(); e == nil || !cleaned {
		t.Fatal("before-start failure cleanup", e)
	}
	var logs bytes.Buffer
	pg = NewDatabase(c.StartParameters(map[string]string{"nonexistent_configuration_setting": "1"}).Logger(&logs))
	if e := pg.Start(); e == nil {
		pg.Close()
		t.Fatal("invalid server setting started")
	}
	if logs.Len() == 0 || pg.Err() == nil {
		t.Fatal("startup diagnostics absent")
	}
	if pg.Logs() == "" {
		t.Fatal("closed instance lost failure diagnostics")
	}
}
func TestUnixSocketAndAmbientEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets")
	}
	c := integrationConfig(t)
	socket, e := os.MkdirTemp("/tmp", "eps-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(socket)
	if c.identity != nil {
		os.Chmod(socket, 0777)
	}
	t.Setenv("PGPASSWORD", "deliberately-wrong")
	t.Setenv("PGHOST", "nonexistent")
	t.Setenv("HOME", "/nonexistent")
	pg := NewDatabase(c.UnixSocket(socket).StartParameters(map[string]string{"max_connections": "30"}).Environment("EP_EXTENSION_ENV=test"))
	if e = pg.Start(); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(pg.ConnectionURL(), "host=") || pg.Logs() == "" {
		t.Fatal("socket endpoint/logs absent")
	}
	if e = pg.Close(); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(socket)
	if e != nil || len(entries) != 0 {
		t.Fatal("owned socket child remains", e)
	}
}
