//go:build !windows

package main

import (
	"os/signal"
	"syscall"
)

// A disconnected JSON consumer must not bypass PostgreSQL cleanup defers.
func prepareSignals() { signal.Ignore(syscall.SIGPIPE) }
