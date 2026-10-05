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
