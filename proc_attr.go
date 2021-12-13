//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package embeddedpostgres

import (
	"syscall"
)

// ProcAttr sets custom attributes for the embedded postgres process
func (c Config) ProcAttr(procAttr *syscall.SysProcAttr) Config {
	c.procAttr = procAttr
	return c
}
