package main

import (
	"log"
    "os"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func main() {
    config := embeddedpostgres.DefaultConfig();
    config = config.BinariesInterpreter(os.Getenv("POSTGRES_INTERPRETER"))

	embeddedPostgres := embeddedpostgres.NewDatabase(config)
	if err := embeddedPostgres.Start(); err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := embeddedPostgres.Stop(); err != nil {
			log.Fatal(err)
		}
	}()
}
