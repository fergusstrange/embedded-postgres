package eptest

import (
	postgres "github.com/fergusstrange/embedded-postgres/v2"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticCleanup(t *testing.T) {
	bin := os.Getenv("EP_TEST_BIN")
	if bin == "" || os.Getenv("EP_SUPERVISOR") == "" {
		t.Skip("integration binaries required")
	}
	var work string
	t.Run("scoped", func(t *testing.T) {
		c := postgres.DefaultConfig().Port(0).BinariesPath(filepath.Dir(bin))
		if version := os.Getenv("EP_TEST_VERSION"); version != "" {
			c = c.Version(postgres.PostgresVersion(version))
		}
		pg := Start(t, c)
		work = pg.Info().WorkDir
		if pg.GetPort() == 0 {
			t.Fatal("dynamic port was not resolved")
		}
	})
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Fatal("test helper did not clean up")
	}
}
