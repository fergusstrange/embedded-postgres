//go:build linux || darwin || freebsd

package filelock

import (
	"errors"
	"os"
	"syscall"
)

func try(f *os.File, exclusive bool) error {
	kind := syscall.LOCK_SH
	if exclusive {
		kind = syscall.LOCK_EX
	}
	return syscall.Flock(int(f.Fd()), kind|syscall.LOCK_NB)
}
func busy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}
