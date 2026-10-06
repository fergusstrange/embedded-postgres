package embeddedpostgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fergusstrange/embedded-postgres/v2/internal/archive"
	"github.com/fergusstrange/embedded-postgres/v2/internal/binaries"
	"github.com/fergusstrange/embedded-postgres/v2/internal/filelock"
)

// BinaryRequest identifies a requested PostgreSQL installation.
type BinaryRequest struct {
	Version      PostgresVersion
	GOOS, GOARCH string
}

// Installation is immutable while acquired. Release is called once, after all
// owned processes stop. It must release leases, not delete caller-owned data.
type Installation struct {
	Dir     string
	Release func() error
}

// BinaryProvider supplies PostgreSQL plus initdb, pg_ctl, psql, and createdb.
// Implementations must support concurrent calls and honour cancellation.
type BinaryProvider interface {
	Acquire(context.Context, BinaryRequest) (*Installation, error)
}

// ProviderFunc adapts a function to BinaryProvider.
type ProviderFunc func(context.Context, BinaryRequest) (*Installation, error)

func (f ProviderFunc) Acquire(ctx context.Context, r BinaryRequest) (*Installation, error) {
	return f(ctx, r)
}

// LocalProvider uses a preinstalled distribution without modifying it.
type LocalProvider string

func (p LocalProvider) Acquire(ctx context.Context, _ BinaryRequest) (*Installation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(string(p))
	if err == nil {
		err = validateInstallation(dir)
	}
	if err != nil {
		return nil, err
	}
	return &Installation{Dir: dir}, nil
}

// DownloadProvider downloads pinned gzip bundles and shares extracted binaries.
// BaseURL mirrors the upstream /VERSION/ASSET layout. Target overrides automatic
// platform detection (for custom musl systems, for example). Client supports
// authenticated mirrors. Offline refuses network requests.
type DownloadProvider struct {
	CacheDir, BaseURL, Target string
	Client                    *http.Client
	Offline                   bool
}

func (p DownloadProvider) Acquire(ctx context.Context, r BinaryRequest) (*Installation, error) {
	if r.GOOS != runtime.GOOS || r.GOARCH != runtime.GOARCH {
		return nil, errors.New("cannot execute binaries for a different host")
	}
	target := p.Target
	var err error
	if target == "" {
		target, err = binaries.Target()
		if err != nil {
			return nil, err
		}
	}
	name, a, err := binaries.Lookup(string(r.Version), target)
	if err != nil {
		return nil, err
	}
	cache, err := cacheDirectory(p.CacheDir)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSuffix(name, ".tar.gz") + "-" + a.SHA256[:16]
	entry := filepath.Join(cache, key)
	lockPath := entry + ".lock"
	for {
		lease, err := filelock.Acquire(ctx, lockPath, false)
		if err != nil {
			return nil, err
		}
		if validCache(entry, a.SHA256) {
			var once sync.Once
			return &Installation{Dir: filepath.Join(entry, strings.TrimSuffix(name, ".tar.gz")), Release: func() error { var e error; once.Do(func() { e = lease.Close() }); return e }}, nil
		}
		lease.Close()
		if p.Offline {
			return nil, fmt.Errorf("PostgreSQL %s is not cached (offline mode)", r.Version)
		}
		lock, err := filelock.Try(lockPath)
		if err != nil {
			if !filelock.IsBusy(err) {
				return nil, err
			}
			// Another installer or a newly acquired reader won the race. Recheck
			// under a shared lease instead of waiting behind active instances.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		err = func() error {
			defer lock.Close()
			if validCache(entry, a.SHA256) {
				return nil
			}
			if _, e := os.Lstat(entry); e == nil {
				return fmt.Errorf("invalid cache entry %s; remove or repair it explicitly", entry)
			} else if !os.IsNotExist(e) {
				return e
			}
			stage, e := os.MkdirTemp(cache, ".install-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(stage)
			downloadURL := a.URL
			if p.BaseURL != "" {
				downloadURL = strings.TrimRight(p.BaseURL, "/") + "/" + url.PathEscape(string(r.Version)) + "/" + name
			}
			f, e := download(ctx, p.Client, downloadURL, a.SHA256, cache, 1<<30)
			if e != nil {
				return e
			}
			defer os.Remove(f.Name())
			defer f.Close()
			if e = archive.Extract(ctx, f, stage); e != nil {
				return e
			}
			if e = validateInstallation(filepath.Join(stage, strings.TrimSuffix(name, ".tar.gz"))); e != nil {
				return e
			}
			if e = os.WriteFile(filepath.Join(stage, ".complete"), []byte(a.SHA256), 0600); e != nil {
				return e
			}
			return os.Rename(stage, entry)
		}()
		if err != nil {
			return nil, err
		}
	}
}
func cacheDirectory(dir string) (string, error) {
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "embedded-postgres", "v2")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return dir, os.MkdirAll(dir, 0700)
}
func validCache(dir, digest string) bool {
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return false
	}
	b, e := os.ReadFile(filepath.Join(dir, ".complete"))
	return e == nil && string(b) == digest
}
func validateInstallation(dir string) error {
	for _, name := range []string{"postgres", "initdb", "pg_ctl", "psql", "createdb"} {
		p := filepath.Join(dir, "bin", platform.Executable(name))
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("required PostgreSQL tool %s: %w", p, err)
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("PostgreSQL tool is not a file: %s", p)
		}
	}
	return nil
}

func download(ctx context.Context, client *http.Client, source, digest, dir string, limit int64) (*os.File, error) {
	if len(digest) != 64 {
		return nil, errors.New("download requires SHA-256 digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			last = err
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			last = fmt.Errorf("download HTTP %d from %s", res.StatusCode, req.URL.Redacted())
			if res.StatusCode != 429 && res.StatusCode < 500 {
				return nil, last
			}
			continue
		}
		f, err := os.CreateTemp(dir, ".download-")
		if err != nil {
			res.Body.Close()
			return nil, err
		}
		hash := sha256.New()
		n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(res.Body, limit+1))
		closeErr := res.Body.Close()
		err = errors.Join(err, closeErr)
		if err == nil && n > limit {
			err = errors.New("download exceeds size limit")
		}
		if err == nil && hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(digest) {
			err = errors.New("download SHA-256 mismatch")
		}
		if err == nil {
			_, err = f.Seek(0, io.SeekStart)
		}
		if err != nil {
			f.Close()
			os.Remove(f.Name())
			return nil, err
		}
		return f, nil
	}
	return nil, fmt.Errorf("download failed: %w", last)
}

// PruneCache removes unused installations. Active leases and unknown entries
// are preserved. Lock files remain so waiters never lock a different inode.
func PruneCache(ctx context.Context, dir string) (int, error) {
	cache, err := cacheDirectory(dir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return count, err
		}
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "postgresql-") {
			continue
		}
		p := filepath.Join(cache, entry.Name())
		marker, e := os.ReadFile(filepath.Join(p, ".complete"))
		if e != nil || len(marker) != 64 {
			continue
		}
		if _, e = hex.DecodeString(string(marker)); e != nil {
			continue
		}
		lock, e := filelock.Try(p + ".lock")
		if e != nil {
			continue
		}
		e = os.RemoveAll(p)
		lock.Close()
		if e != nil {
			return count, e
		}
		count++
	}
	return count, nil
}
