//go:build !linux

package platform

func ProtectExecutableWrite() func() { return func() {} }
