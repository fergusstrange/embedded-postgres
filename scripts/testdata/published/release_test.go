package consumer_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	postgres "github.com/fergusstrange/embedded-postgres/v2"
)

func TestPublishedDefault(t *testing.T) {
	root := os.Getenv("EP_CONSUMER_EVIDENCE")
	if root == "" {
		t.Fatal("missing evidence directory")
	}
	cache, work := filepath.Join(root, "cache"), filepath.Join(root, "work")
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("expected a fresh cache", err)
	}
	if postgres.V18 != "18.6.1" || postgres.V17 != "17.11.1" || postgres.V16 != "16.15.1" || postgres.V15 != "15.19.1" {
		t.Fatal("incorrect published pins")
	}
	cfg := postgres.DefaultConfig().Port(0).Storage(postgres.Storage{CacheDir: cache, WorkDir: work})
	for _, offline := range []bool{false, true} {
		c := cfg
		if offline {
			c = c.Provider(postgres.DownloadProvider{CacheDir: cache, Offline: true})
		}
		pg := postgres.NewDatabase(c)
		if err := pg.Start(); err != nil {
			t.Fatal(err, pg.Logs())
		}
		t.Cleanup(func() { _ = pg.Close() })
		var psql string
		err := filepath.WalkDir(cache, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && (d.Name() == "psql" || d.Name() == "psql.exe") && strings.Contains(path, "postgresql-18.6.1-") {
				psql = path
			}
			return nil
		})
		if err != nil || psql == "" {
			t.Fatal("corrected release cache not found", err)
		}
		if runtime.GOOS == "windows" && !strings.Contains(psql, "x86_64-pc-windows-msvc") {
			t.Fatal("Windows shipping path did not select x64 PostgreSQL", psql)
		}
		out, err := exec.Command(psql, pg.ConnectionURL(), "-Atc", "SELECT 42; SHOW server_version_num;").CombinedOutput()
		if err != nil || strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n")) != "42\n180006" {
			t.Fatal("query", string(out), err)
		}
		port := pg.GetPort()
		if err := pg.Close(); err != nil {
			t.Fatal(err)
		}
		if err := pg.Err(); err != nil {
			t.Fatal(err)
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			conn.Close()
			t.Fatal("database port survived cleanup")
		}
		entries, err := os.ReadDir(work)
		if err != nil || len(entries) != 0 {
			t.Fatal("workspace survived cleanup", entries, err)
		}
		t.Logf("Published module default PostgreSQL 18.6.1: SQL, offline=%v, cleanup passed", offline)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	tag := os.Getenv("EP_CONSUMER_TAG")
	companion := filepath.Join(cache, tag+"-embedded-postgres_"+runtime.GOOS+"_"+runtime.GOARCH+suffix)
	if _, err := os.Stat(companion); err != nil {
		t.Fatal("automatic matching supervisor missing", err)
	}
}
