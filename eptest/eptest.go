// Package eptest binds an embedded PostgreSQL instance to Go tests and suites.
package eptest

import (
	"context"
	"fmt"
	"os"
	"testing"

	postgres "github.com/fergusstrange/embedded-postgres/v2"
)

// Start starts an isolated server and registers cleanup before startup. Defaults
// choose a dynamic port. An explicit Config is honoured, including its port.
func Start(t testing.TB, configs ...postgres.Config) *postgres.EmbeddedPostgres {
	t.Helper()
	c := postgres.DefaultConfig().Port(0)
	if len(configs) > 0 {
		c = configs[0]
	}
	pg := postgres.NewDatabase(c)
	t.Cleanup(func() {
		logs := pg.Logs()
		if err := pg.Close(); err != nil {
			t.Errorf("close embedded PostgreSQL: %v", err)
		}
		if t.Failed() && logs != "" {
			t.Logf("PostgreSQL output:\n%s", logs)
		}
	})
	if err := pg.StartContext(t.Context()); err != nil {
		t.Fatalf("start embedded PostgreSQL: %v", err)
	}
	return pg
}

// Run starts one server for TestMain, calls setup (if non-nil), runs m, and closes
// the server before returning the exit code. Call os.Exit only on the result.
// Independent supervision also covers unrecovered panics in test goroutines.
func Run(m *testing.M, setup func(*postgres.EmbeddedPostgres), configs ...postgres.Config) (code int) {
	c := postgres.DefaultConfig().Port(0)
	if len(configs) > 0 {
		c = configs[0]
	}
	pg := postgres.NewDatabase(c)
	defer func() {
		if err := pg.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close embedded PostgreSQL:", err)
			if code == 0 {
				code = 1
			}
		}
	}()
	if err := pg.StartContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "start embedded PostgreSQL:", err)
		return 1
	}
	if setup != nil {
		setup(pg)
	}
	return m.Run()
}
