package filelock

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestLease(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	a, err := Acquire(context.Background(), p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Acquire(context.Background(), p, false)
	if err != nil {
		t.Fatal(err)
	}
	b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err = Acquire(ctx, p, true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline: %v", err)
	}
	a.Close()
	c, err := Acquire(context.Background(), p, true)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestTryAndInvalidPaths(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	f, e := Try(p)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := Try(p); e == nil {
		other.Close()
		t.Fatal("exclusive lease bypassed")
	}
	f.Close()
	if _, e := Try(filepath.Join(p, "child")); e == nil {
		t.Fatal("bad lock path")
	}
	if _, e := Acquire(t.Context(), filepath.Join(p, "child"), true); e == nil {
		t.Fatal("bad acquire path")
	}
}
