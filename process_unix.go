//go:build !windows
// +build !windows

package embeddedpostgres

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type postgresProcess struct {
	Config Config
	Logger *syncedLogger
	cmd    *exec.Cmd
}

func (pp *postgresProcess) Start(ctx context.Context) error {
	postgresBinary := filepath.Join(pp.Config.binariesPath, "bin/postgres")
	cmd := exec.Command(postgresBinary,
		"-D", pp.Config.dataPath,
		"-p", fmt.Sprintf("%d", pp.Config.port))
	cmd.Stdout = pp.Logger.file
	cmd.Stderr = pp.Logger.file
	pp.cmd = cmd

	if err := pp.cmd.Start(); err != nil {
		return fmt.Errorf("could not start postgres using %s", pp.cmd.String())
	}

	if err := pp.waitForPostmasterReady(ctx, 100*time.Millisecond); err != nil {
		if stopErr := pp.cmd.Process.Signal(syscall.SIGINT); stopErr != nil {
			return fmt.Errorf("unable to stop database casused by error %s", err)
		}

		return err
	}

	return nil
}

func (pp *postgresProcess) waitForPostmasterReady(ctx context.Context, interval time.Duration) (err error) {
	statusTicker := time.NewTicker(interval)
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for database to become available: %w", err)
		case <-statusTicker.C:
			_ = pp.Logger.flush()

			var status *pgStatus

			if pp.cmd.Process == nil {
				return fmt.Errorf("no process found")
			}

			status, err = pgCtlStatus(pp.Config)

			if status != nil && status.Running {
				if status.Pid != pp.cmd.Process.Pid {
					return fmt.Errorf("process running, but for wrong pid, expected %d, got %d", pp.cmd.Process.Pid, status.Pid)
				}

				return nil
			}
		}
	}
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
func (pp *postgresProcess) Stop() error {
	_ = pp.cmd.Process.Signal(syscall.SIGINT)
	return pp.cmd.Wait()
}
