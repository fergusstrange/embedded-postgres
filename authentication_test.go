package embeddedpostgres

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWindowsAuthenticationDiagnostics(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows diagnostic")
	}
	c := integrationConfig(t)
	var hba string
	var original []byte
	c = c.Hooks(Hooks{BeforeStart: []Hook{func(_ context.Context, i InstanceInfo) (func(context.Context) error, error) {
		hba = filepath.Join(i.DataDir, "pg_hba.conf")
		var e error
		original, e = os.ReadFile(hba)
		if e != nil {
			return nil, e
		}
		return nil, os.WriteFile(hba, append([]byte("host all all 127.0.0.1/32 trust\n"), original...), 0600)
	}}})
	pg := NewDatabase(c)
	if e := pg.Start(); e != nil {
		t.Fatal("diagnostic database failed to start")
	}
	defer pg.Close()
	s := pg.active
	if e := os.WriteFile(hba, original, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := pg.command(t.Context(), s, "pg_ctl", "reload", "-D", s.info.DataDir); e != nil {
		t.Fatal("reload failed")
	}
	time.Sleep(300 * time.Millisecond)
	for _, uri := range []bool{false, true} {
		connection := withoutPassword(pg.ConnectionURL())
		if uri {
			connection = pg.ConnectionURL()
		}
		_, e := pg.command(t.Context(), s, "psql", "-X", "-w", "-d", connection, "-Atc", "SELECT 1")
		t.Logf("URI password %v: authentication succeeded=%v", uri, e == nil)
	}
}
