package embeddedpostgres

import "time"

type Config struct {
	version      PostgresVersion
	port         uint32
	database     string
	username     string
	password     string
	runtimePath  string
	startTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		version:      V12_1_0,
		port:         5432,
		database:     "postgres",
		username:     "postgres",
		password:     "postgres",
		startTimeout: 15 * time.Second,
	}
}

func (c Config) Version(version PostgresVersion) Config {
	c.version = version
	return c
}

func (c Config) Port(port uint32) Config {
	c.port = port
	return c
}

func (c Config) Database(database string) Config {
	c.database = database
	return c
}

func (c Config) Username(username string) Config {
	c.username = username
	return c
}

func (c Config) Password(password string) Config {
	c.password = password
	return c
}

func (c Config) RuntimePath(path string) Config {
	c.runtimePath = path
	return c
}

func (c Config) StartTimeout(timeout time.Duration) Config {
	c.startTimeout = timeout
	return c
}

type PostgresVersion string

const (
	V12_1_0  = "12.1.0"
	V11_6_0  = "11.6.0"
	V10_11_0 = "10.11.0"
	V9_6_16  = "9.6.16"
)
