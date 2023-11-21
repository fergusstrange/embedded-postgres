//go:build windows
// +build windows

package embeddedpostgres

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type postgresProcess struct {
	Config Config
	Logger *syncedLogger
}

func encodeOptions(port uint32, parameters map[string]string) string {
	options := []string{fmt.Sprintf("-p %d", port)}
	for k, v := range parameters {
		options = append(options, fmt.Sprintf("-c %s='%s'", k, v))
	}
	return strings.Join(options, " ")
}

// Start
// On Windows, you need to jump through hoops to start the process as a restricted user.
// Postgres won't start as administrator.
// So for now we just use pg_ctl on Windows since it does the hoop jumping.
func (pp *postgresProcess) Start(ctx context.Context) error {
	pgCtlBinary := filepath.Join(pp.Config.binariesPath, "bin/pg_ctl")
	cmd := exec.Command(pgCtlBinary, "start", "-w",
		"-D", pp.Config.dataPath,
		"-o", encodeOptions(pp.Config.port, pp.Config.startParameters))
	cmd.Stdout = pp.Logger.file
	cmd.Stderr = pp.Logger.file

	if err := cmd.Run(); err != nil {
		_ = pp.Logger.flush()
		logContent, _ := readLogsOrTimeout(pp.Logger.file)

		return fmt.Errorf("could not start postgres using %s:\n%s", cmd.String(), string(logContent))
	}

	return nil
}

// Stop will try to stop the Postgres process gracefully returning an error when there were any problems.
// Again, on Windows, we use pg_ctl.
func (pp *postgresProcess) Stop() error {

	pgCtlBinary := filepath.Join(pp.Config.binariesPath, "bin/pg_ctl")
	cmd := exec.Command(pgCtlBinary, "stop", "-w", "-D", pp.Config.dataPath)
	cmd.Stdout = pp.Logger.file
	cmd.Stderr = pp.Logger.file

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not stop postgres using %s", cmd.String())
	}
	return nil
}
