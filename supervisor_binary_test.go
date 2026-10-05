package embeddedpostgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestSupervisorAcquisition(t *testing.T) {
	content := []byte("verified executable fixture")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	name := executable("embedded-postgres-test")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			fmt.Fprintf(w, "%s  %s\n", digest, name)
		} else {
			w.Write(content)
		}
	}))
	defer server.Close()
	cache := t.TempDir()
	for range 2 {
		p, e := cacheSupervisor(t.Context(), cache, "v2.0.0", name, server.URL, server.Client())
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(p)
		if e != nil || string(b) != string(content) {
			t.Fatal("invalid installed binary", e)
		}
	}
	if requests != 2 {
		t.Fatalf("cache downloaded again: %d", requests)
	}
}
func TestSupervisorAcquisitionErrors(t *testing.T) {
	for _, mode := range []string{"status", "missing", "corrupt", "invalid-url", "lock", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "status":
					w.WriteHeader(404)
				case "missing":
					fmt.Fprintln(w, "unknown checksum")
				default:
					fmt.Fprintln(w, strings.Repeat("0", 64), "helper")
				}
			}))
			defer server.Close()
			cache := t.TempDir()
			base := server.URL
			ctx := t.Context()
			if mode == "invalid-url" {
				base = ":bad"
			}
			if mode == "lock" {
				cache = filepath.Join(cache, "missing", "dir")
			}
			if mode == "cancel" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			if _, e := cacheSupervisor(ctx, cache, "v2.0.0", "helper", base, server.Client()); e == nil {
				t.Fatal("accepted", mode)
			}
		})
	}
}
func TestResolveSupervisorAndModuleIdentity(t *testing.T) {
	t.Setenv("EP_SUPERVISOR", "")
	if _, e := resolveSupervisor(t.Context(), DefaultConfig()); e == nil {
		t.Fatal("source build acquired arbitrary release")
	}
	path := filepath.Join(t.TempDir(), "helper")
	os.WriteFile(path, []byte("fixture"), 0755)
	for _, c := range []Config{DefaultConfig().Supervisor(path), DefaultConfig()} {
		t.Setenv("EP_SUPERVISOR", path)
		got, e := resolveSupervisor(t.Context(), c)
		if e != nil || got != path {
			t.Fatal(got, e)
		}
	}
	for _, path := range []string{t.TempDir(), "/no-such-embedded-postgres-helper"} {
		if _, e := resolveSupervisor(t.Context(), DefaultConfig().Supervisor(path)); e == nil {
			t.Fatal("invalid executable")
		}
	}
	info := &debug.BuildInfo{Deps: []*debug.Module{{Path: "other", Version: "v1.0.0"}, {Path: "github.com/fergusstrange/embedded-postgres/v2", Version: "v2.1.0"}}}
	if moduleVersion(info) != "v2.1.0" {
		t.Fatal("version not detected")
	}
	info.Deps[1].Replace = &debug.Module{Path: "local"}
	if moduleVersion(info) != "" {
		t.Fatal("replacement treated as release")
	}
}
func TestCopyInstallationBoundaries(t *testing.T) {
	source := t.TempDir()
	os.Mkdir(filepath.Join(source, "bin"), 0755)
	os.WriteFile(filepath.Join(source, "bin", "tool"), []byte("tool"), 0755)
	target := filepath.Join(t.TempDir(), "copied")
	if e := copyInstallation(t.Context(), source, target); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(target, "bin", "tool")); e != nil || string(b) != "tool" {
		t.Fatal(string(b), e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if e := copyInstallation(ctx, source, t.TempDir()); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if e := copyInstallation(t.Context(), "missing-source", t.TempDir()); e == nil {
		t.Fatal("missing source accepted")
	}
	if e := copyFile("missing", filepath.Join(t.TempDir(), "file"), 0600); e == nil {
		t.Fatal("missing file")
	}
	if e := copyFile(filepath.Join(source, "bin", "tool"), target, 0600); e == nil {
		t.Fatal("overwritten directory")
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("private"), 0600)
	if e := os.Symlink(outside, filepath.Join(source, "escape")); e == nil {
		if e = copyInstallation(t.Context(), source, filepath.Join(t.TempDir(), "copy")); e == nil {
			t.Fatal("escaping symlink copied")
		}
		os.Remove(filepath.Join(source, "escape"))
	} else {
		t.Log("symlink creation unavailable", e)
	}
	if e := os.Symlink(filepath.Join(source, "bin", "tool"), filepath.Join(source, "inside")); e == nil {
		if e = copyInstallation(t.Context(), source, filepath.Join(t.TempDir(), "copy")); e != nil {
			t.Fatal(e)
		}
	}
}
func TestLogTailBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	text := strings.Repeat("x", 100<<10) + "tail"
	os.WriteFile(path, []byte(text), 0600)
	got := logTail(path)
	if len(got) != 32<<10 || !strings.HasSuffix(string(got), "tail") {
		t.Fatal("incorrect bounded tail")
	}
	if len(logTail("missing")) != 0 {
		t.Fatal("missing log")
	}
	if withoutPassword("postgresql://u:secret@localhost/db") != "postgresql://u@localhost/db" || withoutPassword(":broken") != "" {
		t.Fatal("URI sanitization")
	}
}
