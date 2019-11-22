package examples

import (
	"github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"testing"
)

func TestMain(m *testing.M) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		panic(err)
	}
	db, err := connect()
	if err != nil {
		panic(err)
	}
	if errMigration := goose.Up(db.DB, "./migrations"); errMigration != nil {
		panic(errMigration)
	}
	exitCode := m.Run()
	database.Stop()
	os.Exit(exitCode)
}

func Test_SelectOne(t *testing.T) {
	db, err := connect()
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

func Test_Insert(t *testing.T) {
	db, err := connect()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`INSERT INTO tom_beresford_beer_catalogue (name, consumed, rating) VALUES ('kernal', true, 99.99)`); err != nil {
		t.Fatal(err)
	}
}

func connect() (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	return db, err
}
