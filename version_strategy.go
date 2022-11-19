package embeddedpostgres

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// VersionStrategy provides a strategy that can be used to determine which version of Postgres should be used based on
// the operating system, architecture and desired Postgres version.
type VersionStrategy func() (operatingSystem string, architecture string, postgresVersion PostgresVersion)

func defaultVersionStrategy(config Config, goos, arch string, linuxMachineName func() string, shouldUseAlpineLinuxBuild func() bool) VersionStrategy {
	return func() (string, string, PostgresVersion) {
		goos := goos
		arch := arch

		if goos == "linux" {
			// the zonkyio/embedded-postgres-binaries project produces
			// arm binaries with the following name schema:
			// 32bit: arm32v6 / arm32v7
			// 64bit (aarch64): arm64v8
			if arch == "arm64" {
				arch += "v8"
			} else if arch == "arm" {
				machineName := linuxMachineName()
				if strings.HasPrefix(machineName, "armv7") {
					arch += "32v7"
				} else if strings.HasPrefix(machineName, "armv6") {
					arch += "32v6"
				}
			}

			if shouldUseAlpineLinuxBuild() {
				arch += "-alpine"
			}
		}

		// if available, use postgres for macos on arm
		if goos == "darwin" && arch == "arm64" {
			var majorVer, minorVer int

			fmt.Sscanf(string(config.version), "%d.%d", &majorVer, &minorVer)
			if majorVer >= 15 || (majorVer == 14 && minorVer >= 2) {
				arch += "v8"
			} else {
				arch = "amd64"
			}
		}

		return goos, arch, config.version
	}
}

func linuxMachineName() string {
	var uname string

	if output, err := exec.Command("uname", "-m").Output(); err == nil {
		uname = string(output)
	}

	return uname
}

func shouldUseAlpineLinuxBuild() bool {
	_, err := os.Stat("/etc/alpine-release")
	return err == nil
}
