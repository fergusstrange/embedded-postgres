package embeddedpostgres

import "context"

// InstanceInfo is a snapshot. Hooks may change only their instance's private
// work/data files, never the shared binary installation.
type InstanceInfo struct {
	WorkDir, DataDir, BinariesDir, ConnectionURL string
	Port                                         uint32
}

// Hook runs synchronously. It must honour ctx and must not call Start/Stop/Close
// on its own instance. Return cleanup for resources acquired before any error.
type Hook func(context.Context, InstanceInfo) (cleanup func(context.Context) error, err error)

// Hooks are ordered phases. BeforeStart runs after initdb, Ready after an
// authenticated query. Returned cleanups run in reverse order after shutdown,
// including when a later hook or startup fails. Panics are cleaned up then rethrown.
type Hooks struct{ BeforeStart, Ready []Hook }

func (c Config) Hooks(v Hooks) Config {
	c.hooks = Hooks{BeforeStart: append([]Hook(nil), v.BeforeStart...), Ready: append([]Hook(nil), v.Ready...)}
	return c
}

// Environment appends explicit subprocess environment overrides. Identity does
// not inherit root-only HOME/PG variables; connection settings are set separately.
func (c Config) Environment(v ...string) Config {
	c.environment = append([]string(nil), v...)
	return c
}
