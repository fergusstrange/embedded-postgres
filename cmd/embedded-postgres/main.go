// embedded-postgres is the v2 CLI and process supervisor.
package main

import (
	"fmt"
	"github.com/fergusstrange/embedded-postgres/v2/internal/supervisor"
	"os"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "__supervise" {
		if err := supervisor.Run(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "CLI commands will be added in milestone 5")
	os.Exit(2)
}
