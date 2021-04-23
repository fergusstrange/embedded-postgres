package embeddedpostgres

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// VersionStrategy provides a strategy that can be used to determine which version of Postgres should be used based on
// the operating system, architecture and desired Postgres version.
type VersionStrategy func() (operatingSystem string, architecture string, postgresVersion PostgresVersion)

func defaultVersionStrategy(config Config) VersionStrategy {
	return func() (operatingSystem, architecture string, version PostgresVersion) {
		goos := runtime.GOOS
		arch := runtime.GOARCH

		if goos == "linux" {
			// the zonkyio/embedded-postgres-binaries project produces
			// arm binaries with the following name schema:
			// 32bit: arm32v6 / arm32v7
			// 64bit (aarch64): arm64v8
			if arch == "arm64" {
				arch += "v8"
			} else if arch == "arm" {
				if out, err := exec.Command("uname", "-m").Output(); err == nil {
					s := string(out)
					if strings.HasPrefix(s, "armv7") {
						arch += "32v7"
					} else if strings.HasPrefix(s, "armv6") {
						arch += "32v6"
					}
				}
			}
			// check alpine specific build
			if _, err := os.Stat("/etc/alpine-release"); err == nil {
				arch += "-alpine"
			}
		}

		return goos, arch, config.version
	}
}
