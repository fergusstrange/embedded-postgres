package embeddedpostgres

import (
	"fmt"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"io"
	"maps"
	"net"
	"net/url"
	"strconv"
	"time"
)

// PostgresVersion identifies a pinned distribution release. Its first two
// components are the PostgreSQL version; the third is the packaging revision.
type PostgresVersion string

const (
	V18 PostgresVersion = "18.6.0"
	V17 PostgresVersion = "17.11.0"
	V16 PostgresVersion = "16.15.0"
	V15 PostgresVersion = "15.19.0"
)

// User identifies an existing OS account. It is separate from Username.
type User = platform.Identity

// Storage separates reusable binaries, disposable work, and persistent data.
// WorkDir is a parent: the library creates its own child and never deletes it.
// DataDir, when set, is persistent and is never automatically removed.
type Storage struct{ CacheDir, WorkDir, DataDir string }

// Config uses value builders. Maps and slices are copied at API boundaries.
type Config struct {
	version                                     PostgresVersion
	port                                        uint32
	database, username, password                string
	storage                                     Storage
	provider                                    BinaryProvider
	binariesPath, repositoryURL, supervisorPath string
	locale, encoding, socketDir                 string
	startParameters                             map[string]string
	startTimeout, stopTimeout                   time.Duration
	logger                                      io.Writer
	identity                                    *User
	hooks                                       Hooks
	environment                                 []string
}

// DefaultConfig retains port 5432 for existing callers. Use Port(0) for automatic
// allocation. Test helpers and the CLI choose Port(0) by default.
func DefaultConfig() Config {
	return Config{version: V18, port: 5432, database: "postgres", username: "postgres", password: "postgres", locale: "C", encoding: "UTF8", startTimeout: 2 * time.Minute, stopTimeout: 10 * time.Second}
}
func (c Config) Version(v PostgresVersion) Config    { c.version = v; return c }
func (c Config) Port(v uint32) Config                { c.port = v; return c }
func (c Config) Database(v string) Config            { c.database = v; return c }
func (c Config) Username(v string) Config            { c.username = v; return c }
func (c Config) Password(v string) Config            { c.password = v; return c }
func (c Config) Locale(v string) Config              { c.locale = v; return c }
func (c Config) Encoding(v string) Config            { c.encoding = v; return c }
func (c Config) StartTimeout(v time.Duration) Config { c.startTimeout = v; return c }
func (c Config) StopTimeout(v time.Duration) Config  { c.stopTimeout = v; return c }
func (c Config) Logger(v io.Writer) Config           { c.logger = v; return c }
func (c Config) StartParameters(v map[string]string) Config {
	c.startParameters = maps.Clone(v)
	return c
}
func (c Config) Storage(v Storage) Config         { c.storage = v; return c }
func (c Config) Provider(v BinaryProvider) Config { c.provider = v; return c }

// RunAs sets the Unix identity for the supervisor and every PostgreSQL command.
// Root callers must set both UID and GID to nonzero values explicitly.
func (c Config) RunAs(v User) Config { c.identity = &v; return c }

// Supervisor selects the companion CLI executable. Released library versions
// can download their matching companion automatically. Source builds must set it.
func (c Config) Supervisor(path string) Config { c.supervisorPath = path; return c }

// UnixSocket selects a private Unix socket directory and disables TCP.
func (c Config) UnixSocket(dir string) Config { c.socketDir = dir; return c }

// RuntimePath is deprecated: use Storage.WorkDir. The path is now a parent and
// is never erased; each instance receives a uniquely named child.
func (c Config) RuntimePath(v string) Config { c.storage.WorkDir = v; return c }

// CachePath is deprecated: use Storage.CacheDir.
func (c Config) CachePath(v string) Config { c.storage.CacheDir = v; return c }

// DataPath is deprecated: use Storage.DataDir. Existing data is preserved.
func (c Config) DataPath(v string) Config { c.storage.DataDir = v; return c }

// BinariesPath is deprecated: use LocalProvider. The distribution must include
// psql and createdb; legacy minimal Zonky bundles do not contain them.
func (c Config) BinariesPath(v string) Config { c.binariesPath = v; return c }

// BinaryRepositoryURL is deprecated: use DownloadProvider.BaseURL. v2 mirrors
// use /VERSION/ASSET.tar.gz, not the v1 Maven/JAR layout.
func (c Config) BinaryRepositoryURL(v string) Config { c.repositoryURL = v; return c }

// GetConnectionURL describes static configuration. For Port(0), use the running
// instance's ConnectionURL instead; this method deliberately does no I/O.
func (c Config) GetConnectionURL() string { return connectionURL(c, c.port) }
func connectionURL(c Config, port uint32) string {
	u := url.URL{Scheme: "postgresql", User: url.UserPassword(c.username, c.password), Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), Path: "/" + c.database}
	u.RawPath = "/" + url.PathEscape(c.database)
	q := url.Values{"sslmode": {"disable"}}
	if c.socketDir != "" {
		u.Host = ""
		q.Set("host", c.socketDir)
		q.Set("port", fmt.Sprint(port))
	}
	u.RawQuery = q.Encode()
	return u.String()
}
