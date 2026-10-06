//go:build linux || darwin || freebsd

package platform

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestNativeIdentityBounds(t *testing.T) {
	for _, tt := range []struct {
		name  string
		id    Identity
		valid bool
	}{
		{"ordinary", Identity{UID: 1000, GID: 1000}, true},
		{"representable-zero-group", Identity{UID: 1000, GID: 0}, true},
		{"root-user", Identity{UID: 0, GID: 1000}, false},
		{"unchanged-user", Identity{UID: math.MaxUint32, GID: 1000}, false},
		{"unchanged-group", Identity{UID: 1000, GID: math.MaxUint32}, false},
		{"int32-boundary", Identity{UID: math.MaxInt32, GID: math.MaxInt32}, true},
		{"large-user", Identity{UID: math.MaxInt32 + 1, GID: 1000}, strconv.IntSize == 64},
		{"large-group", Identity{UID: 1000, GID: math.MaxInt32 + 1}, strconv.IntSize == 64},
		{"largest-unix-ids", Identity{UID: math.MaxUint32 - 1, GID: math.MaxUint32 - 1}, strconv.IntSize == 64},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uid, gid, err := nativeIdentity(tt.id)
			if (err == nil) != tt.valid {
				t.Fatalf("nativeIdentity(%+v) error = %v, valid = %v", tt.id, err, tt.valid)
			}
			if err == nil && (uid < 0 || gid < 0 || uint64(uid) != uint64(tt.id.UID) || uint64(gid) != uint64(tt.id.GID)) {
				t.Fatalf("identity changed during conversion: %+v -> %d:%d", tt.id, uid, gid)
			}
		})
	}
}

func TestOwnRejectsInvalidIdentityBeforeChown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	ids := []Identity{
		{UID: 0, GID: uint32(os.Getegid())},
		{UID: math.MaxUint32, GID: math.MaxUint32},
		{UID: 1000, GID: math.MaxUint32},
	}
	if strconv.IntSize == 32 {
		ids = append(ids, Identity{UID: math.MaxInt32 + 1, GID: 1000}, Identity{UID: 1000, GID: math.MaxInt32 + 1})
	}
	for _, id := range ids {
		var pathErr *os.PathError
		if err := Own(path, &id); err == nil || errors.As(err, &pathErr) {
			t.Fatalf("Own(%+v) reached chown instead of rejecting the identity: %v", id, err)
		}
		if err := ValidateIdentity(&id); err == nil {
			t.Fatalf("ValidateIdentity(%+v) did not reject the identity: %v", id, err)
		}
	}
}

func TestRootCallerRequiresExplicitGroup(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root identity validation runs in native Linux CI")
	}
	if err := ValidateIdentity(&Identity{UID: 10001}); err == nil || !strings.Contains(err.Error(), "nonzero GID") {
		t.Fatalf("omitted group was not rejected: %v", err)
	}
	if err := ValidateIdentity(&Identity{UID: 10001, GID: 10001}); err != nil {
		t.Fatalf("explicit non-root group was rejected: %v", err)
	}
}

func TestUnprivilegedCallerCanRetainGroupZero(t *testing.T) {
	if os.Getenv("EP_GROUP_ZERO_CHILD") == "1" {
		if os.Geteuid() == 0 || os.Getegid() != 0 {
			t.Fatal("invalid arbitrary-UID fixture")
		}
		if err := ValidateIdentity(&Identity{UID: uint32(os.Geteuid()), GID: 0}); err != nil {
			t.Fatalf("unprivileged caller cannot retain its own group: %v", err)
		}
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("root fixture setup runs in native Linux CI")
	}
	uid, err := strconv.ParseUint(os.Getenv("EP_TEST_UID"), 10, 32)
	if err != nil || uid == 0 {
		t.Skip("EP_TEST_UID must identify a non-root test account")
	}
	// Only the subprocess changes identity. This simulates an existing container
	// identity; the library still rejects selecting group zero from a root caller.
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestUnprivilegedCallerCanRetainGroupZero$")
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: 0}}
	child.Env = append(os.Environ(), "EP_GROUP_ZERO_CHILD=1")
	if dir := os.Getenv("EP_COVERDIR"); dir != "" {
		child.Env = append(child.Env, "GOCOVERDIR="+dir)
	}
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("arbitrary-UID child: %v\n%s", err, output)
	}
}
