//go:build windows

package atomic

import (
	"syscall"
	"unsafe"
)

const (
	movefile_replace_existing = 0x1
	movefile_write_through    = 0x8
)

//sys moveFileEx(lpExistingFileName *uint16, lpNewFileName *uint16, dwFlags uint32) (err error) = MoveFileExW

// Rename atomically replaces the destination file or directory with the
// source.  It is guaranteed to either replace the target file entirely, or not
// change either file.
func Rename(src, dst string) error {
	kernel32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		return err
	}
	defer syscall.FreeLibrary(kernel32)
	moveFileExUnicode, err := syscall.GetProcAddress(kernel32, "MoveFileExW")
	if err != nil {
		return err
	}

	srcString, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}

	dstString, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}

	srcPtr := uintptr(unsafe.Pointer(srcString))
	dstPtr := uintptr(unsafe.Pointer(dstString))

	MOVEFILE_REPLACE_EXISTING := 0x1
	flag := uintptr(MOVEFILE_REPLACE_EXISTING)

	_, _, callErr := syscall.Syscall(uintptr(moveFileExUnicode), 3, srcPtr, dstPtr, flag)
	if callErr != 0 {
		return callErr
	}

	return nil
}
