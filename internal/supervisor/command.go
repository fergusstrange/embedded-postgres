package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"io"
)

// RunCommand supervises initdb and client utilities too. The caller cancels by
// closing its pipe. Only the child group created here is terminated on EOF.
func RunCommand(input io.Reader, output io.Writer) error {
	if err := platform.Guard(); err != nil {
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(input, 1<<20))
	var req Request
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if req.Protocol != Protocol {
		return errors.New("invalid command protocol")
	}
	cmd, release, err := platform.Command(context.Background(), nil, req.Binary, req.Args...)
	if err != nil {
		return err
	}
	cmd.Dir = req.DataDir
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Start()
	release()
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	gone := make(chan struct{})
	go func() { var ignored any; _ = decoder.Decode(&ignored); close(gone) }()
	select {
	case err := <-done:
		return err
	case <-gone:
		_ = platform.Kill(cmd.Process)
		return <-done
	}
}
