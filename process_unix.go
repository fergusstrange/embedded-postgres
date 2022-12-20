//go:build !windows
// +build !windows

package embeddedpostgres

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func (ep *EmbeddedPostgres) startPostgres(ctx context.Context) error {
	postgresBinary := filepath.Join(ep.config.binariesPath, "bin/postgres")
	ep.cmd = exec.Command(postgresBinary,
		"-D", ep.config.dataPath,
		"-p", fmt.Sprintf("%d", ep.config.port))
	ep.cmd.Stdout = ep.syncedLogger.file
	ep.cmd.Stderr = ep.syncedLogger.file

	if err := ep.cmd.Start(); err != nil {
		return fmt.Errorf("could not start postgres using %s", ep.cmd.String())
	}

	if err := ep.waitForPostmasterReady(ctx, 100*time.Millisecond); err != nil {
		if stopErr := ep.cmd.Process.Signal(syscall.SIGINT); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

		return err
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (ep *EmbeddedPostgres) Stop() error {
	if !ep.started {
		return errors.New("server has not been started")
	}

	_ = ep.cmd.Process.Signal(syscall.SIGINT)
	_ = ep.cmd.Wait()

	ep.started = false

	if err := ep.syncedLogger.flush(); err != nil {
		return err
	}

	return nil
}
