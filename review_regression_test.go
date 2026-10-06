package embeddedpostgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStopContextDeadlineDuringStartup(t *testing.T) {
	c := integrationConfig(t)
	entered, release := make(chan struct{}), make(chan struct{})
	c = c.Hooks(Hooks{BeforeStart: []Hook{func(context.Context, InstanceInfo) (func(context.Context) error, error) {
		close(entered)
		<-release
		return nil, nil
	}}})
	pg := NewDatabase(c)
	started := make(chan error, 1)
	go func() { started <- pg.Start() }()
	select {
	case <-entered:
	case err := <-started:
		t.Fatal("startup never reached blocking hook", err)
	case <-time.After(30 * time.Second):
		close(release)
		t.Fatal("startup stalled before hook")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- pg.StopContext(ctx) }()
	select {
	case err := <-stopped:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("StopContext blocked behind startup after its deadline")
	}
	close(release)
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	select {
	case <-pg.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("timed-out stop abandoned queued cleanup")
	}
}

func TestPersistentAuthenticationFailsPromptly(t *testing.T) {
	c := integrationConfig(t)
	parent, err := os.MkdirTemp("", "ep-auth-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	if c.identity != nil {
		if err = os.Chmod(parent, 0755); err != nil {
			t.Fatal(err)
		}
	}
	c = c.DataPath(filepath.Join(parent, "data"))
	pg := NewDatabase(c)
	if err = pg.Start(); err != nil {
		t.Fatal(err)
	}
	if err = pg.Close(); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []Config{c.Password("incorrect-password"), c.Username("nonexistent-role")} {
		pg = NewDatabase(cfg.StartTimeout(8 * time.Second))
		start := time.Now()
		err = pg.Start()
		if err == nil {
			pg.Close()
			t.Fatal("invalid credentials accepted")
		}
		if errors.Is(err, context.DeadlineExceeded) || time.Since(start) >= 8*time.Second || !strings.Contains(err.Error(), "authentication failed") {
			t.Fatalf("permanent authentication failure retried until deadline: %v", err)
		}
		if strings.Contains(err.Error(), cfg.password) {
			t.Fatal("password exposed in failure")
		}
	}
}

func TestPermanentConnectionErrors(t *testing.T) {
	for _, message := range []string{"FATAL:  password authentication failed for user", "fe_sendauth: no password supplied", "FATAL:  role \"missing\" does not exist", "FATAL:  no pg_hba.conf entry", "FATAL:  pg_hba.conf rejects connection"} {
		if !permanentConnectionError(message) {
			t.Error("permanent failure retried", message)
		}
	}
	for _, message := range []string{"connection refused", "FATAL:  the database system is starting up", "listener belongs to a different PostgreSQL instance"} {
		if permanentConnectionError(message) {
			t.Error("transient failure rejected", message)
		}
	}
}

func TestRunAsMissingSocketParent(t *testing.T) {
	c := integrationConfig(t)
	if c.identity == nil {
		t.Skip("root RunAs integration")
	}
	parent := filepath.Join(t.TempDir(), "missing")
	pg := NewDatabase(c.UnixSocket(parent))
	if err := pg.Start(); err == nil || !strings.Contains(err.Error(), "RunAs UnixSocket parent must be an existing directory") {
		pg.Close()
		t.Fatalf("missing RunAs socket parent did not fail clearly: %v", err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatal("created inaccessible caller-owned parent", err)
	}
}

func TestRunAsOmittedGroupFailsBeforeAcquisition(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() != 0 {
		t.Skip("root Unix startup regression")
	}
	acquired := false
	provider := ProviderFunc(func(context.Context, BinaryRequest) (*Installation, error) {
		acquired = true
		return nil, errors.New("unexpected provider acquisition")
	})
	pg := NewDatabase(DefaultConfig().RunAs(User{UID: 10001}).Provider(provider))
	if err := pg.Start(); err == nil || !strings.Contains(err.Error(), "nonzero GID") {
		t.Fatalf("omitted group was not rejected: %v", err)
	}
	if acquired {
		t.Fatal("invalid RunAs identity reached binary acquisition")
	}
}
