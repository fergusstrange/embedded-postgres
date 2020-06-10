package embeddedpostgres

import "time"

// Config maintains the runtime configuration for the Postgres process to be created.
type Config struct {
	version      PostgresVersion
	port         uint32
	database     string
	username     string
	password     string
	runtimePath  string
	locale       string
	startTimeout time.Duration
}

// DefaultConfig provides a default set of configuration to be used "as is" or modified using the provided builders.
// The following can be assumed as defaults:
// Version:      12
// Port:         5432
// Database:     postgres
// Username:     postgres
// Password:     postgres
// StartTimeout: 15 Seconds
func DefaultConfig() Config {
	return Config{
		version:      V12,
		port:         5432,
		database:     "postgres",
		username:     "postgres",
		password:     "postgres",
		startTimeout: 15 * time.Second,
	}
}

// Version will set the Postgres binary version.
func (c Config) Version(version PostgresVersion) Config {
	c.version = version
	return c
}

// Port sets the runtime port that Postgres can be accessed on.
func (c Config) Port(port uint32) Config {
	c.port = port
	return c
}

// Database sets the database name that will be created.
func (c Config) Database(database string) Config {
	c.database = database
	return c
}

// Username sets the username that will be used to connect.
func (c Config) Username(username string) Config {
	c.username = username
	return c
}

// Password sets the password that will be used to connect.
func (c Config) Password(password string) Config {
	c.password = password
	return c
}

// RuntimePath sets the path that will be used for the extracted Postgres runtime and data directory.
func (c Config) RuntimePath(path string) Config {
	c.runtimePath = path
	return c
}

// Locale sets the default locale for initdb
func (c Config) Locale(locale string) Config {
	c.locale = locale
	return c
}

// StartTimeout sets the max timeout that will be used when starting the Postgres process and creating the initial database.
func (c Config) StartTimeout(timeout time.Duration) Config {
	c.startTimeout = timeout
	return c
}

// PostgresVersion represents the semantic version used to fetch and run the Postgres process.
type PostgresVersion string

// Predefined supported Postgres versions.
const (
	V12 = PostgresVersion("12.1.0")
	V11 = PostgresVersion("11.6.0")
	V10 = PostgresVersion("10.11.0")
	V9  = PostgresVersion("9.6.16")
)
