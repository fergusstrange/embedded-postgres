// Package archive extracts bounded gzip archives beneath a private directory.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const MaxExpanded = int64(2 << 30)

// Extract requires an empty, exclusively owned destination. Links are created
// only after regular files, preventing link-based writes even within the root.
func Extract(ctx context.Context, input io.Reader, dest string) error {
	gz, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer gz.Close()
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	tr := tar.NewReader(gz)
	type link struct {
		name, target string
		hard         bool
	}
	var links []link
	var total int64
	seen := map[string]bool{}
	for count := 0; ; count++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		h, e := tr.Next()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return e
		}
		if count >= 100000 {
			return errors.New("archive has too many entries")
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name == "." {
			continue
		}
		if !safe(name) {
			return fmt.Errorf("unsafe archive path %q", h.Name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate archive path %q", name)
		}
		seen[name] = true
		if err = root.MkdirAll(filepath.FromSlash(path.Dir(name)), 0755); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = root.MkdirAll(filepath.FromSlash(name), 0755)
		case tar.TypeReg:
			if h.Size < 0 || h.Size > MaxExpanded-total {
				return errors.New("archive exceeds expanded size limit")
			}
			total += h.Size
			var f *os.File
			mode := os.FileMode(0644)
			if h.Mode&0111 != 0 {
				mode = 0755
			}
			f, err = root.OpenFile(filepath.FromSlash(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, &contextReader{ctx, tr})
			err = errors.Join(err, f.Close())
		case tar.TypeSymlink, tar.TypeLink:
			target := h.Linkname
			if strings.ContainsAny(target, "\\:") || strings.HasPrefix(target, "/") {
				return fmt.Errorf("unsafe archive link %q", target)
			}
			resolved := path.Clean(path.Join(path.Dir(name), target))
			hard := h.Typeflag == tar.TypeLink
			if hard {
				resolved = path.Clean(target)
			}
			if !safe(resolved) {
				return fmt.Errorf("escaping archive link %q", name)
			}
			links = append(links, link{name, target, hard})
		default:
			return fmt.Errorf("unsupported archive entry %q (type %d)", name, h.Typeflag)
		}
		if err != nil {
			return err
		}
	}
	// Force gzip footer/checksum verification even when tar ends early.
	if n, e := io.Copy(io.Discard, io.LimitReader(&contextReader{ctx, gz}, (1<<20)+1)); e != nil || n > 1<<20 {
		if e == nil {
			e = errors.New("excessive trailing archive data")
		}
		err = e
		return err
	}
	for _, l := range links {
		if err = ctx.Err(); err != nil {
			return err
		}
		if l.hard {
			err = root.Link(filepath.FromSlash(l.target), filepath.FromSlash(l.name))
		} else {
			err = root.Symlink(filepath.FromSlash(l.target), filepath.FromSlash(l.name))
		}
		if err != nil {
			return err
		}
	}
	for _, l := range links {
		if _, err = root.Stat(filepath.FromSlash(l.name)); err != nil {
			return fmt.Errorf("invalid archive link %s: %w", l.name, err)
		}
	}
	return nil
}
func safe(s string) bool {
	return s != "" && s != "." && !strings.ContainsAny(s, "\\:") && !path.IsAbs(s) && path.Clean(s) == s && s != ".." && !strings.HasPrefix(s, "../") && filepath.IsLocal(filepath.FromSlash(s))
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
