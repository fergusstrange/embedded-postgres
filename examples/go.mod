module github.com/fergusstrange/embedded-postgres/examples

go 1.13

replace github.com/fergusstrange/embedded-postgres => ../

require (
	github.com/fergusstrange/embedded-postgres v0.0.0
	github.com/jmoiron/sqlx v1.3.5
	github.com/lib/pq v1.10.4
	github.com/pressly/goose/v3 v3.5.3
	go.uber.org/zap v1.21.0
)
