//go:build windows

package platform

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsNativeFailuresReleaseHandles(t *testing.T) {
	failure := syscall.ERROR_ACCESS_DENIED

	handleCount := func() uint32 {
		t.Helper()
		current, _ := syscall.GetCurrentProcess()
		var count uint32
		ok, _, e := kernel.NewProc("GetProcessHandleCount").Call(uintptr(current), uintptr(unsafe.Pointer(&count)))
		if ok == 0 {
			t.Fatal(e)
		}
		return count
	}
	before := handleCount()
	for range 3 {
		for _, kind := range []string{"open", "admin-sid", "power-sid", "restrict"} {
			calls := nativeWindows
			switch kind {
			case "open":
				calls.openToken = func(syscall.Handle, uint32, *syscall.Token) error { return failure }
			case "admin-sid", "power-sid":
				calls.sid = func(value string) (*syscall.SID, error) {
					if (kind == "admin-sid" && value == "S-1-5-32-544") || (kind == "power-sid" && value == "S-1-5-32-547") {
						return nil, failure
					}
					return syscall.StringToSid(value)
				}
			case "restrict":
				calls.restrict = func(syscall.Token, []syscall.SIDAndAttributes, *syscall.Token) (uintptr, error) { return 0, failure }
			}
			release, e := configureWindows(exec.Command("unused"), nil, calls)
			if e == nil {
				release()
				t.Fatal("native token failure ignored", kind)
			}
			if !errors.Is(e, failure) {
				t.Fatal("native failure cause lost", kind, e)
			}
		}
		for _, kind := range []string{"create", "limit", "assign"} {
			calls := nativeWindows
			switch kind {
			case "create":
				calls.create = func() (uintptr, error) { return 0, failure }
			case "limit":
				calls.limit = func(uintptr, *extendedLimits) (uintptr, error) { return 0, failure }
			case "assign":
				calls.assign = func(uintptr, syscall.Handle) (uintptr, error) { return 0, failure }
			}
			// Every case fails before any process can be assigned to the job.
			if e := guardWindows(calls); !errors.Is(e, failure) {
				t.Fatal("job failure cause lost", kind, e)
			}
		}
	}
	if after := handleCount(); after > before+2 {
		t.Fatalf("native failure paths leaked handles: before=%d after=%d", before, after)
	}
}
