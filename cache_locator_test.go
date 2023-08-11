package embeddedpostgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultCacheLocator_NotExists(t *testing.T) {
	locator := defaultCacheLocator("", func() (string, string, PostgresVersion) {
		return "a", "b", "1.2.3"
	})

	cacheLocation, exists := locator()

	assert.Contains(t, cacheLocation, ".embedded-postgres-go/embedded-postgres-binaries-a-b-1.2.3.txz")
	assert.False(t, exists)
}

func Test_defaultCacheLocator_CustomPath(t *testing.T) {
	locator := defaultCacheLocator("/custom/path", func() (string, string, PostgresVersion) {
		return "a", "b", "1.2.3"
	})

	cacheLocation, exists := locator()

	assert.Equal(t, cacheLocation, "/custom/path/embedded-postgres-binaries-a-b-1.2.3.txz")
	assert.False(t, exists)
}
