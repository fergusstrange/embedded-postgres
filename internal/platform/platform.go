// Package platform contains the small OS boundary for process ownership.
package platform

import (
	"context"
	"os/exec"
)

// Identity is an operating-system identity, not a PostgreSQL role.
type Identity struct{ UID, GID uint32 }

// Command applies identity and process isolation before any child can execute.
// The caller must invoke release after Run or Start, even on error.
func Command(ctx context.Context, identity *Identity, name string, args ...string) (*exec.Cmd, func(), error) {
	cmd := exec.CommandContext(ctx, name, args...)
	release, err := configure(cmd, identity)
	if err != nil {
		return nil, nil, err
	}
	cmd.Cancel = func() error { return Kill(cmd.Process) }
	return cmd, release, nil
}
