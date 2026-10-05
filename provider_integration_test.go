package embeddedpostgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fergusstrange/embedded-postgres/v2/internal/binaries"
)

func TestVerifiedConcurrentCache(t *testing.T) {
	archive := os.Getenv("EP_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set EP_TEST_ARCHIVE to the pinned native gzip bundle")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.ServeFile(w, r, archive) }))
	defer server.Close()
	cache := t.TempDir()
	p := DownloadProvider{CacheDir: cache, BaseURL: server.URL}
	req := BinaryRequest{Version: V18, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	var wg sync.WaitGroup
	installs := make(chan *Installation, 3)
	for range 3 {
		wg.Go(func() {
			i, e := p.Acquire(t.Context(), req)
			if e != nil {
				t.Error(e)
				return
			}
			installs <- i
		})
	}
	wg.Wait()
	close(installs)
	if requests.Load() != 1 {
		t.Fatalf("expected one verified download, got %d", requests.Load())
	}
	if n, e := PruneCache(t.Context(), cache); e != nil || n != 0 {
		t.Fatal("active installation pruned", n, e)
	}
	for i := range installs {
		if e := validateInstallation(i.Dir); e != nil {
			t.Error(e)
		}
		if e := i.Release(); e != nil {
			t.Error(e)
		}
		if e := i.Release(); e != nil {
			t.Error("release not idempotent", e)
		}
	}
	p.Offline = true
	i, e := p.Acquire(t.Context(), req)
	if e != nil {
		t.Fatal(e)
	}
	i.Release()
	if n, e := PruneCache(t.Context(), cache); e != nil || n != 1 {
		t.Fatal("unused installation not pruned", n, e)
	}
	if _, e = p.Acquire(t.Context(), req); e == nil {
		t.Fatal("offline miss succeeded")
	}
}
func TestProviderRejectsInvalidCacheAndHost(t *testing.T) {
	req := BinaryRequest{Version: V18, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	cache := t.TempDir()
	p := DownloadProvider{CacheDir: cache, Offline: true}
	foreign := req
	foreign.GOOS = "other"
	if _, e := p.Acquire(t.Context(), foreign); e == nil {
		t.Fatal("foreign binary accepted")
	}
	bad := req
	bad.Version = "14.0.0"
	if _, e := p.Acquire(t.Context(), bad); e == nil {
		t.Fatal("EOL default bundle accepted")
	}
	target, e := binaries.Target()
	if e != nil {
		t.Fatal(e)
	}
	name, a, e := binaries.Lookup(string(V18), target)
	if e != nil {
		t.Fatal(e)
	}
	entry := filepath.Join(cache, strings.TrimSuffix(name, ".tar.gz")+"-"+a.SHA256[:16])
	os.Mkdir(entry, 0700)
	p.Offline = false
	if _, e = p.Acquire(t.Context(), req); e == nil || !strings.Contains(e.Error(), "invalid cache") {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0600)
	p.CacheDir = file
	if _, e = p.Acquire(t.Context(), req); e == nil {
		t.Fatal("file accepted as cache directory")
	}
	if _, e = PruneCache(t.Context(), file); e == nil {
		t.Fatal("file pruned as cache")
	}
	os.WriteFile(filepath.Join(entry, ".complete"), []byte(strings.Repeat("x", 64)), 0600)
	if n, e := PruneCache(t.Context(), cache); e != nil || n != 0 {
		t.Fatal("invalid ownership marker pruned", n, e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e = PruneCache(ctx, cache); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingBody) Close() error             { return errors.New("body close failed") }
func TestDownloadFailuresAndRetries(t *testing.T) {
	digest := strings.Repeat("0", 64)
	for _, value := range []string{"short", strings.Repeat("z", 64)} {
		if _, e := download(t.Context(), nil, "https://unused", value, t.TempDir(), 100); e == nil {
			t.Fatal("invalid digest")
		}
	}
	if _, e := download(t.Context(), nil, ":bad", digest, t.TempDir(), 100); e == nil {
		t.Fatal("invalid URL")
	}
	for _, status := range []int{404, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
			})}
			if _, e := download(t.Context(), client, "https://fixture", digest, t.TempDir(), 100); e == nil {
				t.Fatal("HTTP failure accepted")
			}
			want := 3
			if status == 404 {
				want = 1
			}
			if calls != want {
				t.Fatal(calls, want)
			}
		})
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	if _, e := download(ctx, client, "https://fixture", digest, t.TempDir(), 100); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: failingBody{}}, nil
	})
	dir := t.TempDir()
	if _, e := download(t.Context(), client, "https://fixture", digest, dir, 100); e == nil {
		t.Fatal("truncated body accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed download retained")
	}
	if _, e := download(t.Context(), client, "https://fixture", digest, filepath.Join(dir, "absent"), 100); e == nil {
		t.Fatal("missing destination accepted")
	}
}
