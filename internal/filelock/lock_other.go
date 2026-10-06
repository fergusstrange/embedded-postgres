//go:build !linux && !darwin && !freebsd && !windows

package filelock

import "os"

func try(*os.File, bool) error { return ErrUnsupported }
func busy(error) bool          { return false }
