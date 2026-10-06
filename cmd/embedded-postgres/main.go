// embedded-postgres runs PostgreSQL for tests and supervises its process tree.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fergusstrange/embedded-postgres/v2/internal/cli"
	"github.com/fergusstrange/embedded-postgres/v2/internal/supervisor"
)

func main() { os.Exit(run()) }
func run() int {
	prepareSignals()
	if len(os.Args) == 2 && (os.Args[1] == "__command" || os.Args[1] == "__supervise") {
		var err error
		if os.Args[1] == "__command" {
			err = supervisor.RunCommand(os.Stdin, os.Stdout)
		} else {
			err = supervisor.Run(os.Stdin, os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	ctx, cancel := cli.NotifyContext(context.Background())
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return (cli.App{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv, Executable: executable}).Main(ctx, os.Args[1:])
}
