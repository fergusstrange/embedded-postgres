//go:build linux || darwin || freebsd

package platform

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNativeIdentityBounds(t *testing.T) {
	for _, tt := range []struct {
		name  string
		id    Identity
		valid bool
	}{
		{"ordinary", Identity{UID: 1000, GID: 1000}, true},
		{"root-group", Identity{UID: 1000, GID: 0}, true},
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
