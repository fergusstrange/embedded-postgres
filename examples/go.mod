module github.com/fergusstrange/embedded-postgres/examples

go 1.13

replace github.com/fergusstrange/embedded-postgres => ../

// To avoid CVE CVE-2021-29482
replace github.com/ulikunitz/xz => github.com/ulikunitz/xz v0.5.8

require (
	github.com/fergusstrange/embedded-postgres v0.0.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.8.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pressly/goose v2.6.0+incompatible
	go.uber.org/zap v1.19.1
	google.golang.org/appengine v1.6.7 // indirect
)
