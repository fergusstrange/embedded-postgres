package preset

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

const (
	defaultAppName         = "embedded-postgres"
	defaultFreeBSDPlatform = "freebsd13"
)

// Options customizes the LocalDevelopment preset.
type Options struct {
	Version             embeddedpostgres.PostgresVersion
	Platform            string
	Port                uint32
	Database            string
	Username            string
	Password            string
	CachePath           string
	RuntimePath         string
	DataPath            string
	BinariesPath        string
	BinaryRepositoryURL string
	StartTimeout        time.Duration
	StartParameters     map[string]string
	Logger              io.Writer
}

type resolvedConfig struct {
	version             embeddedpostgres.PostgresVersion
	platform            string
	port                uint32
	database            string
	username            string
	password            string
	cachePath           string
	runtimePath         string
	dataPath            string
	binariesPath        string
	binaryRepositoryURL string
	startTimeout        time.Duration
	startParameters     map[string]string
	logger              io.Writer
}

// LocalDevelopment returns a config with filesystem defaults that work well for
// local macOS development and FreeBSD 13 hosts.
//
// On FreeBSD amd64 it pins the default artifact line to freebsd13 unless
// Options.Platform overrides it. Runtime files are kept under /var/tmp and the
// data directory is placed under /var/db so it survives process restarts.
func LocalDevelopment(appName string, options Options) embeddedpostgres.Config {
	resolved := resolveLocalDevelopment(runtime.GOOS, os.TempDir(), appName, options)

	config := embeddedpostgres.DefaultConfig().
		RuntimePath(resolved.runtimePath).
		DataPath(resolved.dataPath)

	if resolved.version != "" {
		config = config.Version(resolved.version)
	}
	if resolved.platform != "" {
		config = config.Platform(resolved.platform)
	}
	if resolved.port != 0 {
		config = config.Port(resolved.port)
	}
	if resolved.database != "" {
		config = config.Database(resolved.database)
	}
	if resolved.username != "" {
		config = config.Username(resolved.username)
	}
	if resolved.password != "" {
		config = config.Password(resolved.password)
	}
	if resolved.cachePath != "" {
		config = config.CachePath(resolved.cachePath)
	}
	if resolved.binariesPath != "" {
		config = config.BinariesPath(resolved.binariesPath)
	}
	if resolved.binaryRepositoryURL != "" {
		config = config.BinaryRepositoryURL(resolved.binaryRepositoryURL)
	}
	if resolved.startTimeout != 0 {
		config = config.StartTimeout(resolved.startTimeout)
	}
	if resolved.startParameters != nil {
		config = config.StartParameters(resolved.startParameters)
	}
	if resolved.logger != nil {
		config = config.Logger(resolved.logger)
	}

	return config
}

func resolveLocalDevelopment(goos, tempDir, appName string, options Options) resolvedConfig {
	runtimePath, dataPath := defaultPaths(goos, tempDir, appName)

	resolved := resolvedConfig{
		version:             options.Version,
		port:                options.Port,
		database:            options.Database,
		username:            options.Username,
		password:            options.Password,
		cachePath:           options.CachePath,
		runtimePath:         runtimePath,
		dataPath:            dataPath,
		binariesPath:        options.BinariesPath,
		binaryRepositoryURL: options.BinaryRepositoryURL,
		startTimeout:        options.StartTimeout,
		startParameters:     options.StartParameters,
		logger:              options.Logger,
	}

	if goos == "freebsd" {
		resolved.platform = defaultFreeBSDPlatform
	}

	if options.Platform != "" {
		resolved.platform = options.Platform
	}
	if options.RuntimePath != "" {
		resolved.runtimePath = options.RuntimePath
	}
	if options.DataPath != "" {
		resolved.dataPath = options.DataPath
	}

	return resolved
}

func defaultPaths(goos, tempDir, appName string) (string, string) {
	name := sanitizeAppName(appName)

	if goos == "freebsd" {
		runtimeBase := filepath.Join("/var/tmp", name, "embedded-postgres")
		dataBase := filepath.Join("/var/db", name, "embedded-postgres")
		return filepath.Join(runtimeBase, "runtime"), filepath.Join(dataBase, "data")
	}

	base := filepath.Join(tempDir, name, "embedded-postgres")
	return filepath.Join(base, "runtime"), filepath.Join(base, "data")
}

func sanitizeAppName(appName string) string {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return defaultAppName
	}

	replacer := strings.NewReplacer(
		string(os.PathSeparator), "-",
		"/", "-",
		"\\", "-",
		" ", "-",
	)

	return replacer.Replace(appName)
}
