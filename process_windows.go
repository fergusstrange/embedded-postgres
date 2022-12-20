//go:build windows
// +build windows

package embeddedpostgres

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

// startPostgres
// On Windows, you need to jump through hoops to start the process as a restricted user.
// Postgres won't start as administrator.
// So for now we just use pg_ctl on Windows since it does the hoop jumping.
func (ep *EmbeddedPostgres) startPostgres(ctx context.Context) error {
	pgCtlBinary := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	ep.cmd = exec.Command(pgCtlBinary, "start",
		"-D", ep.config.dataPath,
		"-p", fmt.Sprintf("%d", ep.config.port))
	ep.cmd.Stdout = ep.syncedLogger.file
	ep.cmd.Stderr = ep.syncedLogger.file

	if err := ep.cmd.Run(); err != nil {
		return fmt.Errorf("could not start postgres using %s", ep.cmd.String())
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
// Again, on Windows, we use pg_ctl.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return errors.New("server has not been started")
	}

	pgCtlBinary := filepath.Join(ep.config.binariesPath, "bin/pg_ctl")
	ep.cmd = exec.Command(pgCtlBinary, "stop", "-D", ep.config.dataPath)
	ep.cmd.Stdout = ep.syncedLogger.file
	ep.cmd.Stderr = ep.syncedLogger.file

	if err := ep.cmd.Run(); err != nil {
		return fmt.Errorf("could not stop postgres using %s", ep.cmd.String())
	}

	ep.started = false

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	return nil
}
