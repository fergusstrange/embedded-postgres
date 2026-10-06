package platform

import "syscall"

// ProtectExecutableWrite prevents another goroutine's fork from inheriting a
// writable descriptor for a new executable. CLOEXEC closes it only at exec;
// Linux can otherwise reject our launch with ETXTBSY during that interval.
func ProtectExecutableWrite() func() {
	syscall.ForkLock.RLock()
	return syscall.ForkLock.RUnlock
}
