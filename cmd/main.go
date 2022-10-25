package main

import (
	"log"

	embeddedpostgres "github.com/tovala/embedded-postgres"
)

func main() {
	embeddedPostgres := embeddedpostgres.NewDatabase()
	if err := embeddedPostgres.Start(); err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := embeddedPostgres.Stop(); err != nil {
			log.Fatal(err)
		}
	}()
}
