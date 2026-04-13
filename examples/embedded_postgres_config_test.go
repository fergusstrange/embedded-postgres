package examples

import (
	"fmt"
	"os"
	"strconv"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func exampleDatabaseConfig() embeddedpostgres.Config {
	config := embeddedpostgres.DefaultConfig()

	if version := os.Getenv("EMBEDDED_POSTGRES_VERSION"); version != "" {
		config = config.Version(embeddedpostgres.PostgresVersion(version))
	}
	if platform := os.Getenv("EMBEDDED_POSTGRES_PLATFORM"); platform != "" {
		config = config.Platform(platform)
	}
	if binariesPath := os.Getenv("EMBEDDED_POSTGRES_BINARIES_PATH"); binariesPath != "" {
		config = config.BinariesPath(binariesPath)
	}
	if runtimePath := os.Getenv("EMBEDDED_POSTGRES_RUNTIME_PATH"); runtimePath != "" {
		config = config.RuntimePath(runtimePath)
	}
	if dataPath := os.Getenv("EMBEDDED_POSTGRES_DATA_PATH"); dataPath != "" {
		config = config.DataPath(dataPath)
	}
	if repositoryURL := os.Getenv("EMBEDDED_POSTGRES_BINARY_REPOSITORY_URL"); repositoryURL != "" {
		config = config.BinaryRepositoryURL(repositoryURL)
	}
	if port := os.Getenv("EMBEDDED_POSTGRES_PORT"); port != "" {
		if parsedPort, err := strconv.ParseUint(port, 10, 32); err == nil {
			config = config.Port(uint32(parsedPort))
		}
	}

	return config
}

func newExampleDatabase() *embeddedpostgres.EmbeddedPostgres {
	return embeddedpostgres.NewDatabase(exampleDatabaseConfig())
}

func exampleDatabasePort() uint32 {
	if port := os.Getenv("EMBEDDED_POSTGRES_PORT"); port != "" {
		if parsedPort, err := strconv.ParseUint(port, 10, 32); err == nil {
			return uint32(parsedPort)
		}
	}
	return 5432
}

func exampleConnectionString() string {
	return fmt.Sprintf(
		"host=localhost port=%d user=postgres password=postgres dbname=postgres sslmode=disable",
		exampleDatabasePort(),
	)
}
