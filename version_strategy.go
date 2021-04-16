package embeddedpostgres

import (
	"os"
	"runtime"
)

// VersionStrategy provides a strategy that can be used to determine which version of Postgres should be used based on
// the operating system, architecture and desired Postgres version.
type VersionStrategy func() (operatingSystem string, architecture string, postgresVersion PostgresVersion)

func defaultVersionStrategy(config Config) VersionStrategy {
	return func() (operatingSystem, architecture string, version PostgresVersion) {
		goos := runtime.GOOS
		arch := runtime.GOARCH

		// use alpine specific build
		if goos == "linux" {
			if _, err := os.Stat("/etc/alpine-release"); err == nil {
				arch += "-alpine"
			}
		}

		return goos, arch, config.version
	}
}
