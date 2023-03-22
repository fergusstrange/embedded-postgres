package embeddedpostgres

import (
	"errors"
	"os"
	"syscall"
)

// renameOrIgnore will rename the oldpath to the newpath.
//
// On Unix this will be a safe atomic operation.
// On Windows this will do nothing if the new path already exists.
//
// This is only safe to use if you can be sure that the newpath is either missing, or contains the same data as the
// old path.
func renameOrIgnore(oldpath, newpath string) error {
	err := os.Rename(oldpath, newpath)

	// if the error is due to syscall.EEXIST then this is most likely windows, and a race condition with
	// multiple downloads of the file. We can assume that the existing file is the correct one and ignore
	// the error
	if errors.Is(err, syscall.EEXIST) {
		return nil
	}

	return err
}
