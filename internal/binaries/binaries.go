// Package binaries contains the audited, release-pinned PostgreSQL catalogue.
package binaries

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

//go:embed manifest.json
var files embed.FS

type Artifact struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func Lookup(version, target string) (string, Artifact, error) {
	var entries map[string]Artifact
	b, _ := files.ReadFile("manifest.json")
	if err := json.Unmarshal(b, &entries); err != nil {
		return "", Artifact{}, err
	}
	name := "postgresql-" + version + "-" + target + ".tar.gz"
	a, ok := entries[name]
	if !ok {
		return "", a, fmt.Errorf("no pinned PostgreSQL %s bundle for %s; supply a BinaryProvider", version, target)
	}
	return name, a, nil
}
func Target() (string, error) {
	_, err := os.Stat("/etc/alpine-release")
	return targetFor(runtime.GOOS, runtime.GOARCH, err == nil)
}
func targetFor(goos, goarch string, musl bool) (string, error) {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64", "386": "i686", "arm": "armv7", "ppc64le": "powerpc64le", "s390x": "s390x"}[goarch]
	if arch == "" {
		return "", fmt.Errorf("unsupported architecture %s", goarch)
	}
	switch goos {
	case "darwin":
		return arch + "-apple-darwin", nil
	case "windows":
		if goarch != "amd64" && goarch != "arm64" {
			break
		}
		return "x86_64-pc-windows-msvc", nil
	case "linux":
		libc := "gnu"
		if musl {
			libc = "musl"
		}
		if goarch == "arm" {
			libc += "eabihf"
		}
		return arch + "-unknown-linux-" + libc, nil
	}
	return "", fmt.Errorf("no default provider for %s/%s", goos, goarch)
}
