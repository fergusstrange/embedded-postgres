package embeddedpostgres

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsFreeBSDICUCopyHint(t *testing.T) {
	t.Parallel()

	if !needsFreeBSDICUCopyHint(`could not open collator for locale "und": U_FILE_ACCESS_ERROR`) {
		t.Fatal("expected ICU copy hint to be needed")
	}
	if needsFreeBSDICUCopyHint(`database system is ready to accept connections`) {
		t.Fatal("did not expect ICU copy hint to be needed")
	}
}

func TestFreeBSDICUCopyHint(t *testing.T) {
	t.Parallel()

	got := freeBSDICUCopyHint("/opt/embedded-postgres")
	wantPath := filepath.Join("/opt/embedded-postgres", "share", "icu")
	if !strings.Contains(got, wantPath) {
		t.Fatalf("hint %q does not contain %q", got, wantPath)
	}
	if !strings.Contains(got, "cp -R") {
		t.Fatalf("hint %q does not contain copy command", got)
	}
}
