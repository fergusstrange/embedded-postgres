package embeddedpostgres

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInitParametersValidation(t *testing.T) {
	for _, m := range []map[string]string{{"PORT": "1234"}, {"data_directory": "other"}, {"": "x"}, {"a=b": "x"}, {"custom": "\x00"}} {
		if validateConfig(DefaultConfig().InitParameters(m)) == nil {
			t.Errorf("invalid init parameters accepted: %q", m)
		}
	}
	if e := validateConfig(DefaultConfig().Version(V15).InitParameters(map[string]string{"dynamic_shared_memory_type": "mmap"})); e == nil || !strings.Contains(e.Error(), "PostgreSQL 16") {
		t.Errorf("V15 init parameters accepted: %v", e)
	}
	for _, c := range []Config{DefaultConfig().Version(V15), DefaultConfig().Version(V15).InitParameters(map[string]string{}), DefaultConfig().Version(V16).InitParameters(map[string]string{"dynamic_shared_memory_type": "mmap"})} {
		if e := validateConfig(c); e != nil {
			t.Errorf("valid config rejected: %v", e)
		}
	}
}

func TestInitParametersCopiedAndOrdered(t *testing.T) {
	m := map[string]string{"wal_level": "minimal", "dynamic_shared_memory_type": "mmap", "max_wal_senders": "0"}
	c := DefaultConfig().InitParameters(m)
	pg := NewDatabase(c)
	m["wal_level"] = "logical"
	c.initParameters["max_wal_senders"] = "10"
	if pg.config.initParameters["wal_level"] != "minimal" || pg.config.initParameters["max_wal_senders"] != "0" || c.initParameters["wal_level"] != "minimal" {
		t.Fatal("init parameters alias caller maps")
	}
	want := []string{"-c", "dynamic_shared_memory_type=mmap", "-c", "max_wal_senders=0", "-c", "wal_level=minimal"}
	if got := settingArgs(pg.config.initParameters); !slices.Equal(got, want) {
		t.Fatalf("settingArgs = %q, want %q", got, want)
	}
	if got := settingArgs(nil); len(got) != 0 {
		t.Fatalf("settingArgs(nil) = %q", got)
	}
}

// Settings given to initdb are written to postgresql.conf and so survive later
// starts of the same cluster that no longer pass them.
func TestInitParametersPersistInCluster(t *testing.T) {
	c := integrationConfig(t)
	if strings.HasPrefix(string(c.version), "15.") {
		t.Skip("initdb -c requires PostgreSQL 16")
	}
	parent, err := os.MkdirTemp("", "ep-initparams-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	if c.identity != nil {
		os.Chmod(parent, 0755)
	}
	data := filepath.Join(parent, "data")
	c = c.DataPath(data)
	for _, init := range []map[string]string{{"dynamic_shared_memory_type": "mmap"}, nil} {
		pg := NewDatabase(c.InitParameters(init))
		if err = pg.Start(); err != nil {
			t.Fatal(err)
		}
		out, e := pg.command(t.Context(), pg.active, "psql", "-X", "-w", "-d", pg.ConnectionURL(), "-Atc", "SHOW dynamic_shared_memory_type")
		if e != nil || strings.TrimSpace(string(out)) != "mmap" {
			pg.Close()
			t.Fatalf("dynamic_shared_memory_type = %q, %v", out, e)
		}
		if err = pg.Close(); err != nil {
			t.Fatal(err)
		}
	}
	conf, err := os.ReadFile(filepath.Join(data, "postgresql.conf"))
	if err != nil || !strings.Contains(string(conf), "dynamic_shared_memory_type = mmap") {
		t.Fatal("postgresql.conf lacks the init setting", err)
	}
}

// initdb fails on an unknown setting, and the error names initialisation.
func TestInitParametersRejectedByInitdb(t *testing.T) {
	c := integrationConfig(t)
	if strings.HasPrefix(string(c.version), "15.") {
		t.Skip("initdb -c requires PostgreSQL 16")
	}
	pg := NewDatabase(c.InitParameters(map[string]string{"nonexistent_configuration_setting": "1"}))
	if e := pg.Start(); e == nil || !strings.Contains(e.Error(), "initialise PostgreSQL") {
		pg.Close()
		t.Fatal("invalid init setting accepted", e)
	}
}
