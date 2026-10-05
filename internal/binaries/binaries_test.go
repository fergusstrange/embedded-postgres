package binaries

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestPinnedSupportedMatrix(t *testing.T) {
	for _, version := range []string{"15.19.0", "16.15.0", "17.11.0", "18.6.0"} {
		for _, target := range []string{"x86_64-unknown-linux-gnu", "aarch64-unknown-linux-gnu", "x86_64-apple-darwin", "aarch64-apple-darwin", "x86_64-pc-windows-msvc"} {
			name, a, e := Lookup(version, target)
			if e != nil {
				t.Fatal(e)
			}
			sum, e := hex.DecodeString(a.SHA256)
			if e != nil || len(sum) != 32 || !strings.HasSuffix(a.URL, name) {
				t.Fatal("invalid manifest entry", name)
			}
		}
	}
	if _, _, e := Lookup("14.0.0", "x86_64-unknown-linux-gnu"); e == nil {
		t.Fatal("EOL release included")
	}
	if _, e := Target(); e != nil {
		t.Fatal(e)
	}
}

func TestTargetVariants(t *testing.T) {
	for _, tc := range []struct {
		os, arch string
		musl     bool
		want     string
	}{{"linux", "amd64", true, "x86_64-unknown-linux-musl"}, {"linux", "arm", false, "armv7-unknown-linux-gnueabihf"}, {"linux", "arm", true, "armv7-unknown-linux-musleabihf"}, {"darwin", "arm64", false, "aarch64-apple-darwin"}, {"windows", "arm64", false, "x86_64-pc-windows-msvc"}} {
		got, e := targetFor(tc.os, tc.arch, tc.musl)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	for _, tc := range [][2]string{{"plan9", "amd64"}, {"linux", "unknown"}, {"windows", "386"}} {
		if _, e := targetFor(tc[0], tc[1], false); e == nil {
			t.Fatal("unsupported target", tc)
		}
	}
}
