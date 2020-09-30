module github.com/fergusstrange/embedded-postgres/examples

go 1.13

replace github.com/fergusstrange/embedded-postgres => ../

require (
	github.com/fergusstrange/embedded-postgres v0.0.0
	github.com/jmoiron/sqlx v1.2.1-0.20190826204134-d7d95172beb5
	github.com/lib/pq v1.8.0
	github.com/pressly/goose v2.6.0+incompatible
	google.golang.org/appengine v1.6.5 // indirect
)
