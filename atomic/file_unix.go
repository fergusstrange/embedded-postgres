//go:build !windows

package atomic

import (
	"os"
)

// Rename atomically replaces the destination file or directory with the
// source.  It is guaranteed to either replace the target file entirely, or not
// change either file.
func Rename(source, destination string) error {
	return os.Rename(source, destination)
}
