package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func bundle(t *testing.T, hs ...tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, h := range hs {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			tw.Write(bytes.Repeat([]byte{'x'}, int(h.Size)))
		}
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}
func TestExtract(t *testing.T) {
	root := t.TempDir()
	data := bundle(t, tar.Header{Name: "pg/bin/postgres", Mode: 0755, Size: 1, Typeflag: tar.TypeReg})
	if err := Extract(context.Background(), bytes.NewReader(data), root); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(root, "pg/bin/postgres"))
	if err != nil || st.Size() != 1 {
		t.Fatalf("%v %v", st, err)
	}
}
func TestRejectUnsafeArchives(t *testing.T) {
	for _, h := range []tar.Header{{Name: "../outside", Typeflag: tar.TypeReg}, {Name: "/outside", Typeflag: tar.TypeReg}, {Name: "C:/outside", Typeflag: tar.TypeReg}, {Name: "pg/link", Linkname: "../../outside", Typeflag: tar.TypeSymlink}, {Name: "pg/fifo", Typeflag: tar.TypeFifo}} {
		t.Run(h.Name, func(t *testing.T) {
			if err := Extract(context.Background(), bytes.NewReader(bundle(t, h)), t.TempDir()); err == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}
func TestRejectDuplicates(t *testing.T) {
	h := tar.Header{Name: "pg/a", Typeflag: tar.TypeReg}
	if Extract(context.Background(), bytes.NewReader(bundle(t, h, h)), t.TempDir()) == nil {
		t.Fatal("accepted duplicate")
	}
}
func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Extract(ctx, bytes.NewReader(bundle(t)), t.TempDir()) == nil {
		t.Fatal("ignored cancellation")
	}
}
func FuzzExtract(f *testing.F) {
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_ = Extract(context.Background(), bytes.NewReader(data), t.TempDir())
	})
}

func TestArchiveLinksAndConflicts(t *testing.T) {
	root := t.TempDir()
	if e := os.Symlink("target", filepath.Join(root, "probe")); e != nil {
		t.Skip("symlinks require platform privilege")
	}
	os.Remove(filepath.Join(root, "probe"))
	hs := []tar.Header{{Name: ".", Typeflag: tar.TypeDir}, {Name: "pg", Typeflag: tar.TypeDir}, {Name: "pg/file", Typeflag: tar.TypeReg, Size: 4}, {Name: "pg/hard", Typeflag: tar.TypeLink, Linkname: "pg/file"}, {Name: "pg/soft", Typeflag: tar.TypeSymlink, Linkname: "file"}}
	if e := Extract(t.Context(), bytes.NewReader(bundle(t, hs...)), root); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"file", "hard", "soft"} {
		if b, e := os.ReadFile(filepath.Join(root, "pg", name)); e != nil || string(b) != "xxxx" {
			t.Fatal(name, e)
		}
	}
	for _, h := range []tar.Header{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/outside"}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "missing"}, {Name: "link", Typeflag: tar.TypeLink, Linkname: "missing"}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "link"}} {
		if e := Extract(t.Context(), bytes.NewReader(bundle(t, h)), t.TempDir()); e == nil {
			t.Fatal("invalid link accepted", h)
		}
	}
	// Existing files and directory/file collisions must never be overwritten.
	for _, hs := range [][]tar.Header{{{Name: "a", Typeflag: tar.TypeReg}, {Name: "a/b", Typeflag: tar.TypeReg}}, {{Name: "a/b", Typeflag: tar.TypeReg}, {Name: "a", Typeflag: tar.TypeReg}}} {
		if e := Extract(t.Context(), bytes.NewReader(bundle(t, hs...)), t.TempDir()); e == nil {
			t.Fatal("path collision accepted")
		}
	}
}
func TestArchiveCorruptionAndLimits(t *testing.T) {
	if e := Extract(t.Context(), bytes.NewReader(bundle(t)), filepath.Join(t.TempDir(), "missing")); e == nil {
		t.Fatal("missing root accepted")
	}
	data := bundle(t, tar.Header{Name: "file", Typeflag: tar.TypeReg, Size: 4})
	data[len(data)-8] ^= 0xff
	if e := Extract(t.Context(), bytes.NewReader(data), t.TempDir()); e == nil {
		t.Fatal("gzip checksum ignored")
	}
	for _, kind := range []string{"size", "count", "trailing", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			tw := tar.NewWriter(gz)
			switch kind {
			case "size":
				_ = tw.WriteHeader(&tar.Header{Name: "huge", Typeflag: tar.TypeReg, Size: MaxExpanded + 1})
			case "count":
				for range 100001 {
					_ = tw.WriteHeader(&tar.Header{Name: ".", Typeflag: tar.TypeDir})
				}
			case "trailing":
				_ = tw.Close()
				_, _ = gz.Write(bytes.Repeat([]byte{'x'}, (1<<20)+1))
			case "truncated":
				_ = tw.WriteHeader(&tar.Header{Name: "short", Typeflag: tar.TypeReg, Size: 100})
				_, _ = tw.Write([]byte("short"))
			}
			_ = tw.Close()
			_ = gz.Close()
			if e := Extract(t.Context(), bytes.NewReader(b.Bytes()), t.TempDir()); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := contextReader{ctx: ctx, r: bytes.NewReader([]byte("data"))}
	if _, e := r.Read(make([]byte, 4)); e == nil {
		t.Fatal("stream read ignored cancellation")
	}
}
