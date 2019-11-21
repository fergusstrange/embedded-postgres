package main

import (
	"github.com/fergusstrange/embedded-postgres"
	"log"
	"time"
)

func main() {
	embeddedPostgres := embeddedpostgres.NewDatabase()
	if err := embeddedPostgres.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Postgres has started...")
	time.Sleep(5 * time.Second)

	embeddedPostgres.Stop()
}
