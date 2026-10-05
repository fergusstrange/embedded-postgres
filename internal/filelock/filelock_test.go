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
