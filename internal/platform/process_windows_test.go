//go:build windows

package platform

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsRestrictedChildCanOpenItsToken(t *testing.T) {
	depth, _ := strconv.Atoi(os.Getenv("EP_WINDOWS_TOKEN_DEPTH"))
	if depth > 0 {
		current, _ := syscall.GetCurrentProcess()
		var token syscall.Token
		if err := syscall.OpenProcessToken(current, syscall.TOKEN_ALL_ACCESS, &token); err != nil {
			t.Fatal("restricted child cannot reopen its own token", depth, err)
		}
		token.Close()
		for _, value := range []string{"S-1-5-32-544", "S-1-5-32-547"} {
			sid, err := syscall.StringToSid(value)
			if err != nil {
				t.Fatal(err)
			}
			var member uint32
			ok, _, err := advapi.NewProc("CheckTokenMembership").Call(0, uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&member)))
			if ok == 0 || member != 0 {
				t.Fatal("restricted child retained privileged group membership", value, member, err)
			}
		}
	}
	if depth == 2 {
		return
	}
	cmd, release, err := Command(t.Context(), nil, os.Args[0], "-test.run=^TestWindowsRestrictedChildCanOpenItsToken$", "-test.v")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cmd.Env = append(cmd.Env, "EP_WINDOWS_TOKEN_DEPTH="+strconv.Itoa(depth+1))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("nested restricted child failed: %v\n%s", err, out)
	}
}

func TestWindowsUserDACLRejectsInvalidToken(t *testing.T) {
	if err := nativeUserDACL(0); err == nil {
		t.Fatal("invalid token accepted")
	}
}

func TestWindowsNativeFailuresReleaseHandles(t *testing.T) {
	failure := syscall.ERROR_ACCESS_DENIED

	// Process-wide handle counts include runtime threads and unrelated activity.
	// Audit the exact resources acquired by this call instead, and require each
	// native CloseHandle to succeed. No unrelated-handle allowance is needed.
	audit := func(calls windowsCalls) (windowsCalls, func()) {
		live := make(map[syscall.Handle]bool)
		track := func(handle syscall.Handle) {
			t.Helper()
			if live[handle] {
				t.Fatal("handle acquired twice without closing", handle)
			}
			live[handle] = true
		}
		open := calls.openToken
		calls.openToken = func(process syscall.Handle, access uint32, token *syscall.Token) error {
			err := open(process, access, token)
			if err == nil {
				track(syscall.Handle(*token))
			}
			return err
		}
		restrict := calls.restrict
		calls.restrict = func(original syscall.Token, denied []syscall.SIDAndAttributes, token *syscall.Token) (uintptr, error) {
			ok, err := restrict(original, denied, token)
			if ok != 0 {
				track(syscall.Handle(*token))
			}
			return ok, err
		}
		create := calls.create
		calls.create = func() (uintptr, error) {
			handle, err := create()
			if handle != 0 {
				track(syscall.Handle(handle))
			}
			return handle, err
		}
		closeHandle := calls.close
		calls.close = func(handle syscall.Handle) error {
			t.Helper()
			if !live[handle] {
				t.Fatal("closing unowned or already closed handle", handle)
			}
			if err := closeHandle(handle); err != nil {
				t.Fatal("native handle close failed", handle, err)
			}
			delete(live, handle)
			return nil
		}
		return calls, func() {
			t.Helper()
			if len(live) != 0 {
				t.Fatal("native handles left open", live)
			}
		}
	}

	for range 3 {
		for _, kind := range []string{"open", "admin-sid", "power-sid", "restrict", "secure", "success"} {
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
			case "secure":
				calls.secure = func(syscall.Token) error { return failure }
			}
			calls, assertClosed := audit(calls)
			release, e := configureWindows(exec.Command("unused"), nil, calls)
			if kind == "success" {
				if e != nil {
					t.Fatal("native token creation failed", e)
				}
				release()
				assertClosed()
				continue
			}
			if e == nil {
				release()
				t.Fatal("native token failure ignored", kind)
			}
			assertClosed()
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
			calls, assertClosed := audit(calls)
			// Every case fails before any process can be assigned to the job.
			if e := guardWindows(calls); !errors.Is(e, failure) {
				t.Fatal("job failure cause lost", kind, e)
			}
			assertClosed()
		}
	}
}
