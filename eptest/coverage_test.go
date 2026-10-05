package eptest

import (
	"context"
	"errors"
	postgres "github.com/fergusstrange/embedded-postgres/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/coverage"
	"testing"
	"time"
)

var suitePostgres *postgres.EmbeddedPostgres

func TestMain(m *testing.M) {
	if dir := os.Getenv("EP_COVERDIR"); dir != "" {
		_ = os.Setenv("GOCOVERDIR", dir)
	}
	if mode := os.Getenv("EP_EPT_SUITE"); mode != "" {
		c := postgres.DefaultConfig().Port(0).BinariesPath(filepath.Dir(os.Getenv("EP_TEST_BIN")))
		if v := os.Getenv("EP_TEST_VERSION"); v != "" {
			c = c.Version(postgres.PostgresVersion(v))
		}
		if mode == "startup-failure" {
			c = c.Port(65536)
		}
		if mode == "cleanup-failure" {
			c = c.Hooks(postgres.Hooks{Ready: []postgres.Hook{func(context.Context, postgres.InstanceInfo) (func(context.Context) error, error) {
				return func(context.Context) error { return errors.New("intentional cleanup failure") }, nil
			}}})
		}
		code := Run(m, func(pg *postgres.EmbeddedPostgres) {
			suitePostgres = pg
			_ = os.WriteFile(os.Getenv("EP_EPT_WORK"), []byte(pg.Info().WorkDir), 0600)
		}, c)
		if dir := os.Getenv("EP_COVERDIR"); dir != "" {
			_ = coverage.WriteMetaDir(dir)
			_ = coverage.WriteCountersDir(dir)
		}
		os.Exit(code)
	}
	code := m.Run()
	if dir := os.Getenv("EP_COVERDIR"); dir != "" {
		_ = coverage.WriteMetaDir(dir)
		_ = coverage.WriteCountersDir(dir)
	}
	os.Exit(code)
}
func TestSuiteChild(t *testing.T) {
	mode := os.Getenv("EP_EPT_SUITE")
	if mode == "" {
		t.Skip("suite subprocess only")
	}
	if suitePostgres == nil || suitePostgres.GetPort() == 0 {
		t.Fatal("suite endpoint unavailable")
	}
	if mode == "test-failure" {
		t.Fatal("intentional test failure")
	}
}
func TestSuiteWrapperLifecycle(t *testing.T) {
	if os.Getenv("EP_TEST_BIN") == "" || os.Getenv("EP_SUPERVISOR") == "" {
		t.Skip("integration binaries required")
	}
	for _, mode := range []string{"success", "test-failure", "startup-failure", "cleanup-failure"} {
		t.Run(mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "work")
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSuiteChild$")
			cmd.Env = append(os.Environ(), "EP_EPT_SUITE="+mode, "EP_EPT_WORK="+file)
			output, err := cmd.CombinedOutput()
			if (err == nil) != (mode == "success") {
				t.Fatalf("exit status: %v\n%s", err, output)
			}
			work, e := os.ReadFile(file)
			if mode != "startup-failure" {
				if e != nil {
					t.Fatal(e)
				}
				if _, e = os.Stat(string(work)); !os.IsNotExist(e) {
					t.Fatal("suite did not clean up", e)
				}
			}
		})
	}
}

func TestScopedFailureChild(t *testing.T) {
	mode := os.Getenv("EP_EPT_SCOPED")
	if mode == "" {
		t.Skip("scoped subprocess only")
	}
	c := postgres.DefaultConfig().Port(0).BinariesPath(filepath.Dir(os.Getenv("EP_TEST_BIN")))
	if v := os.Getenv("EP_TEST_VERSION"); v != "" {
		c = c.Version(postgres.PostgresVersion(v))
	}
	if mode == "startup" {
		c = c.Port(65536)
	}
	if mode == "cleanup" {
		c = c.Hooks(postgres.Hooks{Ready: []postgres.Hook{func(context.Context, postgres.InstanceInfo) (func(context.Context) error, error) {
			return func(context.Context) error { return errors.New("intentional cleanup failure") }, nil
		}}})
	}
	pg := Start(t, c)
	_ = os.WriteFile(os.Getenv("EP_EPT_WORK"), []byte(pg.Info().WorkDir), 0600)
	if mode == "failure" {
		t.Error("intentional assertion failure")
	}
}
func TestScopedFailureCleanup(t *testing.T) {
	if os.Getenv("EP_TEST_BIN") == "" || os.Getenv("EP_SUPERVISOR") == "" {
		t.Skip("integration binaries required")
	}
	for _, mode := range []string{"startup", "cleanup", "failure"} {
		t.Run(mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "work")
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScopedFailureChild$")
			cmd.Env = append(os.Environ(), "EP_EPT_SCOPED="+mode, "EP_EPT_WORK="+file)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("test failure was not reported: %s", output)
			}
			if mode != "startup" {
				work, e := os.ReadFile(file)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = os.Stat(string(work)); !os.IsNotExist(e) {
					t.Fatal("scoped failure retained work", e)
				}
			}
		})
	}
}
