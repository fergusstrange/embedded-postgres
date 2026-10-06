//go:build windows

package filelock

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var lockFile = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func try(f *os.File, exclusive bool) error {
	flags := uintptr(1)
	if exclusive {
		flags |= 2
	}
	var overlap syscall.Overlapped
	ok, _, err := lockFile.Call(f.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlap)))
	if ok == 0 {
		return err
	}
	return nil
}
func busy(err error) bool { return errors.Is(err, syscall.Errno(33)) }
