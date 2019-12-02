package embeddedpostgres

import (
	"fmt"
	"os"
	"path/filepath"
)

type CacheLocator func() (string, bool)

func defaultCacheLocator(versionStrategy VersionStrategy) CacheLocator {
	return func() (string, bool) {
		cacheDirectory := ".embedded-postgres-go"
		if userHome, err := os.UserHomeDir(); err == nil {
			cacheDirectory = filepath.Join(userHome, ".embedded-postgres-go")
		}
		operatingSystem, architecture, version := versionStrategy()
		cacheLocation := filepath.Join(cacheDirectory,
			fmt.Sprintf("embedded-postgres-binaries-%s-%s-%s.txz",
				operatingSystem,
				architecture,
				version))
		info, err := os.Stat(cacheLocation)
		if err != nil {
			return cacheLocation, os.IsExist(err) && !info.IsDir()
		}
		return cacheLocation, !info.IsDir()
	}
}
