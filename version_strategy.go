package embeddedpostgres

import "runtime"

// VersionStrategy provides a strategy that can be used to determine which version of Postgres should be used based on
// the operating system, architecture and desired Postgres version.
type VersionStrategy func() (operatingSystem string, architecture string, postgresVersion PostgresVersion)

func defaultVersionStrategy(config Config) VersionStrategy {
	return func() (operatingSystem, architecture string, version PostgresVersion) {
		return runtime.GOOS, runtime.GOARCH, config.version
	}
}
