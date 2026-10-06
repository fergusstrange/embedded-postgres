package custom

import (
	"context"
	"errors"
	postgres "github.com/fergusstrange/embedded-postgres/v2"
	"testing"
)

func TestMigrationAdapter(t *testing.T) {
	want := errors.New("migration failed")
	closed := false
	hook := Migrations(func(ctx context.Context, uri string) (func(context.Context) error, error) {
		if uri != "postgresql://example" {
			t.Fatal(uri)
		}
		return func(context.Context) error { closed = true; return nil }, want
	})
	close, err := hook(context.Background(), postgres.InstanceInfo{ConnectionURL: "postgresql://example"})
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err = close(context.Background()); err != nil || !closed {
		t.Fatal("cleanup contract")
	}
}
