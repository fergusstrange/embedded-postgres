package platform

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestIdentityBoundary(t *testing.T) {
	if e := ValidateIdentity(&Identity{UID: 0}); e == nil {
		t.Fatal("root/unsupported identity accepted")
	}
	if runtime.GOOS != "windows" {
		if e := ValidateIdentity(&Identity{UID: ^uint32(0), GID: 1}); e == nil {
			t.Fatal("unchanged-ID sentinel accepted")
		}
		if os.Geteuid() != 0 {
			if e := ValidateIdentity(&Identity{UID: uint32(os.Geteuid() + 1), GID: uint32(os.Getegid())}); e == nil {
				t.Fatal("identity change allowed without root")
			}
		}
	}
	if _, _, e := Command(t.Context(), &Identity{}, "unused"); e == nil {
		t.Fatal("invalid identity launched")
	}
	if e := Own("unused", nil); e != nil {
		t.Fatal(e)
	}
	if e := Kill(nil); e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" {
		if e := Own("missing-file", &Identity{UID: 1, GID: 1}); e == nil {
			t.Fatal("chown missing file")
		}
	}
}
func TestCommandEnvironmentAndCancellation(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("non-root native command test")
	}
	t.Setenv("EP_PLATFORM_ENV", "propagated")
	cmd, release, e := Command(t.Context(), nil, os.Args[0], "-test.run=^TestEnvironmentChild$")
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	output, e := cmd.CombinedOutput()
	if e != nil || !strings.Contains(string(output), "propagated") {
		t.Fatal("restricted child lost environment", e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd, release, e = Command(ctx, nil, os.Args[0])
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	if e = cmd.Run(); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" {
		cmd, release, e = Command(t.Context(), &Identity{UID: uint32(os.Geteuid()), GID: uint32(os.Getegid())}, os.Args[0], "-test.run=^TestEnvironmentChild$")
		if e != nil {
			t.Fatal(e)
		}
		defer release()
		if e = cmd.Run(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestEnvironmentChild(t *testing.T) {
	if v := os.Getenv("EP_PLATFORM_ENV"); v != "" {
		os.Stdout.WriteString(v)
	}
}
