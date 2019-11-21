module github.com/fergusstrange/embedded-postgres/examples

replace github.com/fergusstrange/embedded-postgres => ../

go 1.13

require (
	github.com/fergusstrange/embedded-postgres v0.0.0-00010101000000-000000000000 // indirect
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/pressly/goose v2.6.0+incompatible
	google.golang.org/appengine v1.6.5 // indirect
)
