package embeddedpostgres

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadWithoutContentLength(t *testing.T) {
	data := []byte("verified archive")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.(http.Flusher).Flush(); w.Write(data) }))
	defer srv.Close()
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	f, err := download(context.Background(), srv.Client(), srv.URL, digest, t.TempDir(), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()
	if _, err = download(context.Background(), srv.Client(), srv.URL, fmt.Sprintf("%064d", 0), t.TempDir(), 100); err == nil {
		t.Fatal("bad checksum accepted")
	}
}
func TestDownloadLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("too big")) }))
	defer srv.Close()
	if _, e := download(context.Background(), srv.Client(), srv.URL, fmt.Sprintf("%x", sha256.Sum256([]byte("too big"))), t.TempDir(), 1); e == nil {
		t.Fatal("oversize accepted")
	}
}
func TestPrunePreservesUnknown(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "postgresql-my-files")
	os.Mkdir(unknown, 0700)
	n, err := PruneCache(context.Background(), dir)
	if err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err = os.Stat(unknown); err != nil {
		t.Fatal(err)
	}
}
