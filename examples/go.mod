module github.com/fergusstrange/embedded-postgres/examples

go 1.18

replace github.com/fergusstrange/embedded-postgres => ../

require (
	github.com/fergusstrange/embedded-postgres v0.0.0
	github.com/jmoiron/sqlx v1.3.5
	github.com/lib/pq v1.10.4
	github.com/pressly/goose/v3 v3.0.1
	go.uber.org/zap v1.21.0
)

require (
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
)
