package embeddedpostgres

import (
	"os"
	"testing"
)

// go test assigns its own GOCOVERDIR. Instrumented executables use a separate
// directory so their metadata is merged explicitly with native unit profiles.
func TestMain(m *testing.M) {
	if dir := os.Getenv("EP_COVERDIR"); dir != "" {
		_ = os.Setenv("GOCOVERDIR", dir)
	}
	os.Exit(m.Run())
}
