module github.com/vegaprotocol/embedded-postgres

go 1.13

require (
	github.com/fergusstrange/embedded-postgres v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.8.0
	github.com/stretchr/testify v1.6.1
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8
)

replace github.com/fergusstrange/embedded-postgres => github.com/vegaprotocol/embedded-postgres v1.13.0
