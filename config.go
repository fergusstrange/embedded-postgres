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
	V12_0_0  = "12.0.0"
	V11_6_0  = "11.6.0"
	V11_5_0  = "11.5.0"
	V11_4_0  = "11.4.0"
	V11_3_0  = "11.3.0"
	V11_2_0  = "11.2.0"
	V11_1_0  = "11.1.0"
	V11_0_0  = "11.0.0"
	V10_11_0 = "10.11.0"
	V10_10_0 = "10.10.0"
	V10_9_0  = "10.9.0"
	V10_8_0  = "10.8.0"
	V10_7_0  = "10.7.0"
	V10_6_0  = "10.6.0"
	V10_5_0  = "10.5.0"
	V10_4_0  = "10.4.0"
	V9_6_16  = "9.6.16"
	V9_6_15  = "9.6.15"
	V9_6_14  = "9.6.14"
	V9_6_13  = "9.6.13"
	V9_6_12  = "9.6.12"
	V9_6_11  = "9.6.11"
	V9_6_10  = "9.6.10"
	V9_6_9   = "9.6.9"
	V9_5_20  = "9.5.20"
	V9_5_19  = "9.5.19"
	V9_5_18  = "9.5.18"
	V9_5_17  = "9.5.17"
	V9_5_16  = "9.5.16"
	V9_5_15  = "9.5.15"
	V9_5_14  = "9.5.14"
	V9_5_13  = "9.5.13"
	V9_4_25  = "9.4.25"
	V9_4_24  = "9.4.24"
	V9_4_23  = "9.4.23"
	V9_4_22  = "9.4.22"
	V9_4_21  = "9.4.21"
	V9_4_20  = "9.4.20"
	V9_4_19  = "9.4.19"
	V9_4_18  = "9.4.18"
	V9_3_25  = "9.3.25"
	V9_3_24  = "9.3.24"
	V9_3_23  = "9.3.23"
)
