package embeddedpostgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestReadinessRejectsAnotherInstance(t *testing.T) {
	c := integrationConfig(t)
	pg := NewDatabase(c)
	if err := pg.Start(); err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	owned := pg.active
	// Probe an actual healthy server with valid credentials but another launch ID.
	other := &session{supervisorPath: owned.supervisorPath, instanceID: "another-launch", info: owned.info, passPath: owned.passPath, logPath: owned.logPath, exited: make(chan struct{})}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err := pg.ready(ctx, other, c)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("accepted another instance: %v", err)
	}
	if err = pg.ready(t.Context(), owned, c); err != nil {
		t.Fatal("foreign probe affected the owned server", err)
	}
	if validateConfig(c.StartParameters(map[string]string{"EMBEDDED_POSTGRES.INSTANCE_ID": "override"})) == nil {
		t.Fatal("launch identity override accepted")
	}
}

type panickingLogWriter struct{}

func (panickingLogWriter) Write([]byte) (int, error) { panic("logger panic") }

func TestLoggerPanicReleasesResources(t *testing.T) {
	c := integrationConfig(t)
	released := false
	dir := c.binariesPath
	c = c.BinariesPath("").Logger(panickingLogWriter{}).Provider(ProviderFunc(func(ctx context.Context, request BinaryRequest) (*Installation, error) {
		i, err := LocalProvider(dir).Acquire(ctx, request)
		if err == nil {
			i.Release = func() error { released = true; return nil }
		}
		return i, err
	}))
	pg := NewDatabase(c)
	if err := pg.Start(); err != nil {
		t.Fatal(err)
	}
	work := pg.Info().WorkDir
	func() {
		defer func() {
			if recover() != "logger panic" {
				t.Error("logger panic was not propagated")
			}
		}()
		_ = pg.Close()
	}()
	if !released || pg.Err() == nil {
		t.Fatal("logger panic abandoned the binary lease or error")
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Fatal("logger panic retained workspace", err)
	}
}
