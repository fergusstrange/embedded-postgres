package embeddedpostgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func integrationConfig(t *testing.T) Config {
	t.Helper()
	bin := os.Getenv("EP_TEST_BIN")
	helper := os.Getenv("EP_SUPERVISOR")
	if bin == "" || helper == "" {
		t.Skip("set EP_TEST_BIN and EP_SUPERVISOR")
	}
	c := DefaultConfig().Port(0).BinariesPath(filepath.Dir(bin)).Supervisor(helper)
	if uid := os.Getenv("EP_TEST_UID"); uid != "" {
		u, _ := strconv.ParseUint(uid, 10, 32)
		g, _ := strconv.ParseUint(os.Getenv("EP_TEST_GID"), 10, 32)
		c = c.RunAs(User{UID: uint32(u), GID: uint32(g)})
	}
	return c
}
func TestConnectionEscaping(t *testing.T) {
	c := DefaultConfig().Username("a:@").Password("p/@:").Database("db/a ?")
	u, e := url.Parse(c.GetConnectionURL())
	if e != nil {
		t.Fatal(e)
	}
	pw, _ := u.User.Password()
	if u.User.Username() != "a:@" || pw != "p/@:" || u.Path != "/db/a ?" || !strings.Contains(u.EscapedPath(), "%2F") {
		t.Fatal(u)
	}
}
func TestConfigCopiesMap(t *testing.T) {
	m := map[string]string{"max_connections": "50"}
	c := DefaultConfig().StartParameters(m)
	m["max_connections"] = "1"
	if c.startParameters["max_connections"] != "50" {
		t.Fatal("mutable config map")
	}
}
func TestValidationBeforeIO(t *testing.T) {
	pg := NewDatabase(DefaultConfig().Port(65536))
	if pg.Start() == nil {
		t.Fatal("bad port accepted")
	}
	if pg.Close() != nil {
		t.Fatal("Close not idempotent")
	}
	if !errors.Is(pg.Stop(), ErrServerNotStarted) {
		t.Fatal("Stop compatibility")
	}
}
func TestParallelLifecycle(t *testing.T) {
	c := integrationConfig(t)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			pg := NewDatabase(c.Database("test-quote\" db"))
			if err := pg.Start(); err != nil {
				t.Error(err)
				return
			}
			if pg.GetPort() == 0 || pg.ConnectionURL() == "" {
				t.Error("missing endpoint")
			}
			work := pg.Info().WorkDir
			if err := pg.Close(); err != nil {
				t.Error(err)
			}
			if _, err := os.Stat(work); !os.IsNotExist(err) {
				t.Error("workspace remains")
			}
			if err := pg.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}
func TestLifetimeCancellation(t *testing.T) {
	c := integrationConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pg := NewDatabase(c)
	if err := pg.StartContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	done := pg.Done()
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("lifetime cancellation did not clean up")
	}
}
func TestPersistentDataNeverErased(t *testing.T) {
	c := integrationConfig(t)
	parent, err := os.MkdirTemp("", "ep-persist-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	if c.identity != nil {
		os.Chmod(parent, 0755)
	}
	data := filepath.Join(parent, "data")
	c = c.DataPath(data)
	for range 2 {
		pg := NewDatabase(c)
		if err = pg.Start(); err != nil {
			t.Fatal(err)
		}
		if err = pg.Close(); err != nil {
			t.Fatal(err)
		}
	}
	pg := NewDatabase(c.Version(V17))
	if err = pg.Start(); err == nil {
		pg.Close()
		t.Fatal("wrong major accepted")
	}
	if _, err = os.Stat(filepath.Join(data, "PG_VERSION")); err != nil {
		t.Fatal("data removed", err)
	}
}
func TestNonemptyDirectoryPreserved(t *testing.T) {
	c := integrationConfig(t)
	parent, err := os.MkdirTemp("", "ep-userdata-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	path := filepath.Join(parent, "precious")
	os.WriteFile(path, []byte("keep"), 0600)
	pg := NewDatabase(c.DataPath(parent))
	if err = pg.Start(); err == nil {
		pg.Close()
		t.Fatal("non-cluster accepted")
	}
	if b, e := os.ReadFile(path); e != nil || string(b) != "keep" {
		t.Fatal("caller data modified")
	}
}
func TestHooksFailureAndPanicCleanup(t *testing.T) {
	for _, panicHook := range []bool{false, true} {
		t.Run(strconv.FormatBool(panicHook), func(t *testing.T) {
			c := integrationConfig(t)
			var work string
			cleaned := false
			pg := NewDatabase(c.Hooks(Hooks{Ready: []Hook{func(_ context.Context, i InstanceInfo) (func(context.Context) error, error) {
				work = i.WorkDir
				return func(context.Context) error { cleaned = true; return nil }, nil
			}, func(context.Context, InstanceInfo) (func(context.Context) error, error) {
				if panicHook {
					panic("hook panic")
				}
				return nil, errors.New("hook failed")
			}}}))
			func() {
				defer func() {
					if r := recover(); r != nil && !panicHook {
						t.Error(r)
					}
				}()
				if e := pg.Start(); e == nil {
					pg.Close()
					t.Error("hook succeeded")
				}
			}()
			if !cleaned {
				t.Error("cleanup skipped")
			}
			if _, e := os.Stat(work); !os.IsNotExist(e) {
				t.Error("workspace remains")
			}
		})
	}
}

func TestProviderReleaseOnFailure(t *testing.T) {
	c := integrationConfig(t)
	released := 0
	p := ProviderFunc(func(context.Context, BinaryRequest) (*Installation, error) {
		return &Installation{Dir: filepath.Dir(os.Getenv("EP_TEST_BIN")), Release: func() error { released++; return nil }}, nil
	})
	pg := NewDatabase(c.BinariesPath("").Provider(p).Hooks(Hooks{Ready: []Hook{func(context.Context, InstanceInfo) (func(context.Context) error, error) {
		return nil, errors.New("setup failed")
	}}}))
	if err := pg.Start(); err == nil {
		t.Fatal("expected hook failure")
	}
	pg.Close()
	if released != 1 {
		t.Fatalf("release count %d", released)
	}
}
