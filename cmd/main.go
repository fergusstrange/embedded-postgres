package main

import (
	"github.com/fergusstrange/embedded-postgres"
	"log"
)

func main() {
	embeddedPostgres := embeddedpostgres.NewDatabase()
	if err := embeddedPostgres.Start(); err != nil {
		log.Fatal(err)
	}

	defer func() {
		embeddedPostgres.Stop()
	}()
}
