package preset

import (
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/assert"
)

func TestResolveLocalDevelopment_DarwinDefaults(t *testing.T) {
	resolved := resolveLocalDevelopment("darwin", "/tmp", "my app", Options{})

	assert.Equal(t, "", resolved.platform)
	assert.Equal(t, filepath.Join("/tmp", "my-app", "embedded-postgres", "runtime"), resolved.runtimePath)
	assert.Equal(t, filepath.Join("/tmp", "my-app", "embedded-postgres", "data"), resolved.dataPath)
}

func TestResolveLocalDevelopment_FreeBSDDefaults(t *testing.T) {
	resolved := resolveLocalDevelopment("freebsd", "/tmp", "my app", Options{})

	assert.Equal(t, "freebsd13", resolved.platform)
	assert.Equal(t, filepath.Join("/var/tmp", "my-app", "embedded-postgres", "runtime"), resolved.runtimePath)
	assert.Equal(t, filepath.Join("/var/db", "my-app", "embedded-postgres", "data"), resolved.dataPath)
}

func TestResolveLocalDevelopment_OptionsOverrideDefaults(t *testing.T) {
	resolved := resolveLocalDevelopment("freebsd", "/tmp", "my app", Options{
		Version:             embeddedpostgres.V17,
		Platform:            "freebsd14",
		Port:                5544,
		Database:            "appdb",
		Username:            "appuser",
		Password:            "secret",
		CachePath:           "/cache",
		RuntimePath:         "/runtime",
		DataPath:            "/data",
		BinariesPath:        "/binaries",
		BinaryRepositoryURL: "https://repo.local/maven2",
		StartTimeout:        45 * time.Second,
		StartParameters:     map[string]string{"max_connections": "200"},
	})

	assert.Equal(t, embeddedpostgres.V17, resolved.version)
	assert.Equal(t, "freebsd14", resolved.platform)
	assert.Equal(t, uint32(5544), resolved.port)
	assert.Equal(t, "appdb", resolved.database)
	assert.Equal(t, "appuser", resolved.username)
	assert.Equal(t, "secret", resolved.password)
	assert.Equal(t, "/cache", resolved.cachePath)
	assert.Equal(t, "/runtime", resolved.runtimePath)
	assert.Equal(t, "/data", resolved.dataPath)
	assert.Equal(t, "/binaries", resolved.binariesPath)
	assert.Equal(t, "https://repo.local/maven2", resolved.binaryRepositoryURL)
	assert.Equal(t, 45*time.Second, resolved.startTimeout)
	assert.Equal(t, map[string]string{"max_connections": "200"}, resolved.startParameters)
}

func TestSanitizeAppName(t *testing.T) {
	assert.Equal(t, defaultAppName, sanitizeAppName("   "))
	assert.Equal(t, "my-app-name", sanitizeAppName("my/app name"))
	assert.Equal(t, "windows-path", sanitizeAppName("windows\\path"))
}
