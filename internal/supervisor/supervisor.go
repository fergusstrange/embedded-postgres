// Package supervisor owns a PostgreSQL process independently of its Go caller.
package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
)

const Protocol = 1

type Request struct {
	Protocol        int           `json:"protocol"`
	Binary          string        `json:"binary"`
	Args            []string      `json:"args"`
	DataDir         string        `json:"data_dir"`
	LogPath         string        `json:"log_path"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}
type Event struct {
	Protocol int    `json:"protocol"`
	PID      int    `json:"pid,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Run requires a dedicated pipe on input. EOF means the owner died or requested
// shutdown. The pipe is not inherited by PostgreSQL, so parent death closes it.
func Run(input io.Reader, output io.Writer) error {
	if err := platform.Guard(); err != nil {
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(input, 1<<20))
	var req Request
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if req.Protocol != Protocol || req.ShutdownTimeout <= 0 || req.ShutdownTimeout > time.Minute {
		return errors.New("invalid supervisor request")
	}
	log, err := os.OpenFile(req.LogPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd, release, err := platform.Command(context.Background(), nil, req.Binary, req.Args...)
	if err != nil {
		return err
	}
	cmd.Dir = req.DataDir
	cmd.Stdout = log
	cmd.Stderr = log
	err = cmd.Start()
	release()
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), req.ShutdownTimeout)
		defer cancel()
		ctl := filepath.Join(filepath.Dir(req.Binary), platform.Executable("pg_ctl"))
		shutdown, release, e := platform.Command(ctx, nil, ctl, "stop", "-D", req.DataDir, "-m", "fast", "-w", "-t", fmt.Sprint(max(1, int(req.ShutdownTimeout.Seconds()))))
		if e == nil {
			shutdown.Dir = req.DataDir
			shutdown.Stdout = log
			shutdown.Stderr = log
			e = shutdown.Run()
			release()
		}
		select {
		case <-done:
			return nil
		default:
		}
		if e != nil || ctx.Err() != nil {
			_ = platform.Kill(cmd.Process)
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			_ = platform.Kill(cmd.Process)
			<-done
			return nil
		}
	}
	if err = json.NewEncoder(output).Encode(Event{Protocol: Protocol, PID: cmd.Process.Pid}); err != nil {
		return errors.Join(err, stop())
	}
	ownerGone := make(chan struct{})
	go func() { var ignored any; _ = decoder.Decode(&ignored); close(ownerGone) }()
	select {
	case <-ownerGone:
		return stop()
	case err := <-done:
		if err == nil {
			return errors.New("PostgreSQL exited before shutdown")
		}
		return fmt.Errorf("PostgreSQL exited: %w", err)
	}
}
