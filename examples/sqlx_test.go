package examples

import (
	"github.com/fergusstrange/embeddedpostgres"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose"
	"testing"
)

func TestMain(m *testing.M) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		panic(err)
	}

	exitCode := m.Run()

	database.Stop()

	os.Exit(exitCode)
}

func Test_Migration(t *testing.T) {
	db, err := sqlx.Connect("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	if errMigration := goose.Up(db.DB, "./migrations"); errMigration != nil {
		t.Fatal(err)
	}
}

func Test_SelectOne(t *testing.T) {
	db, err := sqlx.Connect("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	rows := make([]int32, 0)
	err = db.Select(&rows, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatal("Expected one row returned")
	}
}
