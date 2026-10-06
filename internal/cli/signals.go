package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type interruption struct{ signal os.Signal }

func (e interruption) Error() string { return e.signal.String() + " signal received" }

// NotifyContext records which signal interrupted the CLI, for forwarding and
// conventional shell exit codes. It never installs handlers in the library.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case s := <-signals:
			cancel(interruption{s})
		case <-ctx.Done():
		}
	}()
	return ctx, func() { signal.Stop(signals); cancel(nil) }
}
func interruptionSignal(ctx context.Context) os.Signal {
	if cause, ok := context.Cause(ctx).(interruption); ok {
		return cause.signal
	}
	return syscall.SIGTERM
}
func interruptionCode(ctx context.Context) int {
	return 128 + int(interruptionSignal(ctx).(syscall.Signal))
}
