//go:build !windows
// +build !windows

package fileutil

import (
	"os"
	"path/filepath"
)

// RenameAndSync will do an os.Rename followed by fsync to ensure the rename
// is recorded
func RenameAndSync(oldpath, newpath string) error {
	err := os.Rename(oldpath, newpath)
	if err != nil {
		return err
	}

	oldparent, newparent := filepath.Dir(oldpath), filepath.Dir(newpath)
	err = fsync(newparent)
	if oldparent != newparent {
		if err1 := fsync(oldparent); err == nil {
			err = err1
		}
	}
	return err
}

func fsync(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}
