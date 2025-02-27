package examples

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
)

func Test_GooseMigrations(t *testing.T) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := connect(database.GetConnectionURL())
	if err != nil {
		t.Fatal(err)
	}

	if err := goose.Up(db.DB, "./migrations"); err != nil {
		t.Fatal(err)
	}
}

func Test_ZapioLogger(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}

	w := &zapio.Writer{Log: logger}

	database := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Logger(w))
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := connect(database.GetConnectionURL())
	if err != nil {
		t.Fatal(err)
	}

	if err := goose.Up(db.DB, "./migrations"); err != nil {
		t.Fatal(err)
	}
}

func Test_Sqlx_SelectOne(t *testing.T) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := connect(database.GetConnectionURL())
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

func Test_UnixSocket_Sqlx_SelectOne(t *testing.T) {
	database := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().WithoutTcp())
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := connect(database.GetConnectionURL())
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

func Test_ManyTestsAgainstOneDatabase(t *testing.T) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	db, err := connect(database.GetConnectionURL())
	if err != nil {
		t.Fatal(err)
	}

	if err := goose.Up(db.DB, "./migrations"); err != nil {
		t.Fatal(err)
	}

	tests := []func(t *testing.T){
		func(t *testing.T) {
			rows := make([]BeerCatalogue, 0)
			if err := db.Select(&rows, "SELECT * FROM beer_catalogue WHERE UPPER(name) = UPPER('Elvis Juice')"); err != nil {
				t.Fatal(err)
			}

			if len(rows) != 0 {
				t.Fatalf("expected 0 rows but got %d", len(rows))
			}
		},
		func(t *testing.T) {
			_, err := db.Exec(`INSERT INTO beer_catalogue (name, consumed, rating) VALUES ($1, $2, $3)`,
				"Kernal",
				true,
				99.32)
			if err != nil {
				t.Fatal(err)
			}

			actualBeerCatalogue := make([]BeerCatalogue, 0)
			if err := db.Select(&actualBeerCatalogue, "SELECT * FROM beer_catalogue WHERE id = 2"); err != nil {
				t.Fatal(err)
			}

			expectedBeerCatalogue := BeerCatalogue{
				ID:       2,
				Name:     "Kernal",
				Consumed: true,
				Rating:   99.32,
			}
			if !reflect.DeepEqual(expectedBeerCatalogue, actualBeerCatalogue[0]) {
				t.Fatalf("expected %+v did not match actual %+v", expectedBeerCatalogue, actualBeerCatalogue)
			}
		},
	}

	for testNumber, test := range tests {
		t.Run(fmt.Sprintf("%d", testNumber), test)
	}
}

func Test_SimpleHttpWebApp(t *testing.T) {
	database := embeddedpostgres.NewDatabase()
	if err := database.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := database.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	request := httptest.NewRequest("GET", "/beer-catalogue?name=Punk%20IPA", nil)
	recorder := httptest.NewRecorder()

	NewApp().router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 but receieved %d", recorder.Code)
	}

	expectedPayload := `[{"id":1,"name":"Punk IPA","consumed":true,"rating":68.29}]`
	actualPayload := recorder.Body.String()

	if actualPayload != expectedPayload {
		t.Fatalf("expected %+v but receieved %+v", expectedPayload, actualPayload)
	}
}

func connect(u string) (*sqlx.DB, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	q := parsed.Query()
	if q.Get("sock") == "" {
		q.Set("sslmode", "disable")
	}
	parsed.RawQuery = q.Encode()

	db, err := sqlx.Connect("postgres", parsed.String())
	return db, err
}
