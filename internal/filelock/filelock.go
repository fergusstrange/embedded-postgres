// Package filelock provides process-scoped shared/exclusive cache leases.
package filelock

import (
	"context"
	"errors"
	"os"
	"time"
)

func Acquire(ctx context.Context, path string, exclusive bool) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		err = try(f, exclusive)
		if err == nil {
			return f, nil
		}
		if !busy(err) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
func Try(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = try(f, true); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

var ErrUnsupported = errors.New("file locking unsupported on this OS")

// IsBusy reports contention with another process-held lease.
func IsBusy(err error) bool { return busy(err) }
