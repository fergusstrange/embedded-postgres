// Package custom contains compiled extension patterns; it adds no core dependencies.
package custom

import (
	"context"
	postgres "github.com/fergusstrange/embedded-postgres/v2"
	"net/http"
)

// Offline uses an already-prefetched cache. It never falls back to networking.
func Offline(cache string) postgres.BinaryProvider {
	return postgres.DownloadProvider{CacheDir: cache, Offline: true}
}

// Mirror reuses the standard verified extraction and cache implementation with
// a caller-owned transport (proxy, client certificate, authentication, etc.).
func Mirror(cache, baseURL string, transport http.RoundTripper) postgres.BinaryProvider {
	return postgres.DownloadProvider{CacheDir: cache, BaseURL: baseURL, Client: &http.Client{Transport: transport}}
}

// Migrations adapts a caller-owned migration library without importing its driver
// into embedded-postgres. The caller supplies the connection pool cleanup.
func Migrations(run func(context.Context, string) (func(context.Context) error, error)) postgres.Hook {
	return func(ctx context.Context, info postgres.InstanceInfo) (func(context.Context) error, error) {
		return run(ctx, info.ConnectionURL)
	}
}

// ExtensionDistribution uses a complete caller-supplied PostgreSQL installation
// containing the extension's compiled libraries, control files, and SQL scripts.
// prepare can run CREATE EXTENSION using the caller's chosen database driver.
func ExtensionDistribution(dir string, prepare postgres.Hook) postgres.Config {
	return postgres.DefaultConfig().Port(0).Provider(postgres.LocalProvider(dir)).Hooks(postgres.Hooks{Ready: []postgres.Hook{prepare}})
}
