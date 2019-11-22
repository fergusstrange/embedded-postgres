module github.com/fergusstrange/embedded-postgres/examples

go 1.13

replace github.com/fergusstrange/embedded-postgres => ../

require (
	github.com/fergusstrange/embedded-postgres v0.0.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/pressly/goose v2.6.0+incompatible
)
