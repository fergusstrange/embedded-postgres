//go:build windows
// +build windows

package fileutil

import (
	"os"
)

// RenameAndSync will do an os.Rename followed by fsync to ensure the rename
// is recorded
func RenameAndSync(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
