//go:build !windows

package atomic

import (
	"fmt"
	"os"
)

// Rename atomically replaces the destination file or directory with the
// source.  It is guaranteed to either replace the target file entirely, or not
// change either file.
func Rename(source, destination string) error {
	fmt.Println("=>=>=>=>", "Replace UNIX")
	return os.Rename(source, destination)
}
