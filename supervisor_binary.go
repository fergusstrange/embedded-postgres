package embeddedpostgres

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/fergusstrange/embedded-postgres/v2/internal/filelock"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// ReleaseVersion is set for CLI builds; Go consumers use module build metadata.
var ReleaseVersion = "v2.0.0-dev"

func resolveSupervisor(ctx context.Context, c Config) (string, error) {
	selected := c.supervisorPath
	if selected == "" {
		selected = os.Getenv("EP_SUPERVISOR")
	}
	if selected != "" {
		p, err := filepath.Abs(selected)
		if err != nil {
			return "", err
		}
		st, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() {
			return "", errors.New("supervisor must be an executable file")
		}
		return p, nil
	}
	version := ""
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, d := range b.Deps {
			if d.Path == "github.com/fergusstrange/embedded-postgres/v2" && d.Replace == nil {
				version = d.Version
			}
		}
	}
	if !regexp.MustCompile(`^v2\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`).MatchString(version) {
		return "", errors.New("source build requires Config.Supervisor(path) or EP_SUPERVISOR; build ./cmd/embedded-postgres first")
	}
	cache, err := cacheDirectory(c.storage.CacheDir)
	if err != nil {
		return "", err
	}
	name := executable("embedded-postgres_" + runtime.GOOS + "_" + runtime.GOARCH)
	path := filepath.Join(cache, version+"-"+name)
	lock, err := filelock.Acquire(ctx, path+".lock", true)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if st, e := os.Stat(path); e == nil && st.Mode().IsRegular() {
		return path, nil
	}
	base := "https://github.com/fergusstrange/embedded-postgres/releases/download/" + version
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/checksums.txt", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("supervisor checksum HTTP %d", res.StatusCode)
	}
	digest := ""
	scanner := bufio.NewScanner(io.LimitReader(res.Body, 1<<20))
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			digest = f[0]
		}
	}
	if err = scanner.Err(); err != nil {
		return "", err
	}
	if digest == "" {
		return "", errors.New("release missing supervisor checksum")
	}
	f, err := download(ctx, client, base+"/"+name, digest, cache, 64<<20)
	if err != nil {
		return "", err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(temp, 0755); err != nil {
		return "", err
	}
	if err = os.Rename(temp, path); err != nil {
		return "", err
	}
	return path, nil
}
func withoutPassword(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}
func copyFile(source, dest string, mode os.FileMode) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}
func copyInstallation(ctx context.Context, source, dest string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			inside, err := filepath.Rel(source, resolved)
			if err != nil || !filepath.IsLocal(inside) {
				return fmt.Errorf("binary symlink escapes installation: %s", rel)
			}
			if filepath.IsAbs(link) {
				link, err = filepath.Rel(filepath.Dir(target), filepath.Join(dest, inside))
				if err != nil {
					return err
				}
			}
			return os.Symlink(link, target)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported binary file: %s", path)
		}
		return copyFile(path, target, info.Mode().Perm()&0755)
	})
}
