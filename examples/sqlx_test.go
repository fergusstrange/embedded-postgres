package examples

import (
	"github.com/fergusstrange/embeddedpostgres"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose"
	"testing"
)

func Test(t *testing.T) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	db, err := sqlx.Connect("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		database.Stop()
		t.Fatal(err)
	}

	if errMigration := goose.Up(db.DB, "./migrations"); errMigration != nil {
		database.Stop()
		t.Fatal(err)
	}

	database.Stop()
}
