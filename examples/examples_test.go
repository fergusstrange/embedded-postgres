package examples_test

import (
	"context"
	"fmt"
	postgres "github.com/fergusstrange/embedded-postgres/v2"
	"github.com/fergusstrange/embedded-postgres/v2/eptest"
	"testing"
)

// These examples are compiled in CI. The matching integration tests exercise
// the lifecycle with real binaries without downloading during example discovery.
func ExampleNewDatabase() {
	pg := postgres.NewDatabase(postgres.DefaultConfig().Port(0))
	if err := pg.StartContext(context.Background()); err != nil {
		panic(err)
	}
	defer pg.Close()
	fmt.Println(pg.ConnectionURL())
}
func Example_testHelper() {
	_ = func(t *testing.T) { t.Parallel(); pg := eptest.Start(t); t.Log(pg.ConnectionURL()) }
}
