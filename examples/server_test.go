package examples

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

type BeerCatalogue struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Consumed bool    `json:"consumed"`
	Rating   float64 `json:"rating"`
}

type App struct {
	router *http.ServeMux
}

func (a *App) Start() error {
	return http.ListenAndServe("localhost:8080", a.router)
}

func NewApp() *App {
	db, err := sqlx.Connect("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	if err := goose.Up(db.DB, "./migrations"); err != nil {
		log.Fatal(err)
	}

	router := http.NewServeMux()
	router.HandleFunc("/beer-catalogue", GetBeer(db))

	return &App{router: router}
}

func GetBeer(db *sqlx.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if beerName := r.URL.Query().Get("name"); beerName != "" {
			beers := make([]BeerCatalogue, 0)
			if err := db.Select(&beers, "SELECT * FROM beer_catalogue WHERE UPPER(name) = UPPER($1)", beerName); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			jsonPayload, err := json.Marshal(beers)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if _, err := w.Write(jsonPayload); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
	}
}
