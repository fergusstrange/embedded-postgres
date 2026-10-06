//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"
)

func TestConsoleInheritanceHelper(t *testing.T) {
	mode := os.Getenv("EP_CONSOLE_TEST")
	if mode == "" {
		return
	}
	var pid uint32
	count, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&pid)), 1)
	if mode == "detached" {
		if count != 0 {
			t.Fatalf("background child inherited a console (%d processes)", count)
		}
		return
	}
	if count == 0 {
		t.Fatalf("parent fixture has no console: %v", err)
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConsoleInheritanceHelper$")
	child.Env = append(os.Environ(), "EP_CONSOLE_TEST=detached")
	detach(child)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("detached child: %v\n%s", err, output)
	}
}

func TestBackgroundDoesNotInheritConsole(t *testing.T) {
	parent := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConsoleInheritanceHelper$")
	parent.Env = append(os.Environ(), "EP_CONSOLE_TEST=attached")
	parent.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x10, HideWindow: true} // CREATE_NEW_CONSOLE
	if output, err := parent.CombinedOutput(); err != nil {
		t.Fatalf("console inheritance regression: %v\n%s", err, output)
	}
}
