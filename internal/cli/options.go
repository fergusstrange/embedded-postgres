// Package cli implements the stable command-line and JSON protocol surface.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	ep "github.com/fergusstrange/embedded-postgres/v2"
)

// Options is the JSON configuration schema. Flags override environment, which
// overrides the file. Durations use Go syntax, for example "30s".
type Options struct {
	Version      string            `json:"postgres_version"`
	Port         uint              `json:"port"`
	Database     string            `json:"database"`
	Username     string            `json:"username"`
	Password     string            `json:"password"`
	CacheDir     string            `json:"cache_dir"`
	WorkDir      string            `json:"work_dir"`
	DataDir      string            `json:"data_dir"`
	Binaries     string            `json:"binaries"`
	Mirror       string            `json:"mirror"`
	User         string            `json:"user"`
	SocketDir    string            `json:"socket_dir"`
	StartTimeout string            `json:"start_timeout"`
	StopTimeout  string            `json:"stop_timeout"`
	Parameters   map[string]string `json:"parameters"`
	Offline      bool              `json:"offline"`
	JSON         bool              `json:"json"`
	ParentStdin  bool              `json:"parent_stdin"`
	StateFile    string            `json:"state_file"`
}

func parse(args []string, getenv func(string) string, stderr io.Writer) (Options, []string, error) {
	o := Options{Version: string(ep.V18), Database: "postgres", Username: "postgres", StartTimeout: "2m", StopTimeout: "10s"}
	config := getenv("EP_CONFIG")
	// Locate the config before parsing so every flag remains an override.
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "-config" {
			if i+1 >= len(args) {
				return o, nil, errors.New("--config needs a path")
			}
			config = args[i+1]
		} else if strings.HasPrefix(arg, "--config=") {
			config = strings.TrimPrefix(arg, "--config=")
		}
	}
	if config != "" {
		f, err := os.Open(config)
		if err != nil {
			return o, nil, err
		}
		d := json.NewDecoder(io.LimitReader(f, 1<<20))
		d.DisallowUnknownFields()
		err = d.Decode(&o)
		if err == nil {
			var extra any
			if d.Decode(&extra) != io.EOF {
				err = errors.New("config must contain one JSON object")
			}
		}
		f.Close()
		if err != nil {
			return o, nil, fmt.Errorf("config: %w", err)
		}
	}
	stringsEnv := map[string]*string{"VERSION": &o.Version, "DATABASE": &o.Database, "USERNAME": &o.Username, "PASSWORD": &o.Password, "CACHE_DIR": &o.CacheDir, "WORK_DIR": &o.WorkDir, "DATA_DIR": &o.DataDir, "BINARIES": &o.Binaries, "MIRROR": &o.Mirror, "USER": &o.User, "SOCKET_DIR": &o.SocketDir, "START_TIMEOUT": &o.StartTimeout, "STOP_TIMEOUT": &o.StopTimeout, "STATE_FILE": &o.StateFile}
	for k, p := range stringsEnv {
		if v := getenv("EP_" + k); v != "" {
			*p = v
		}
	}
	if v := getenv("EP_PORT"); v != "" {
		n, e := strconv.ParseUint(v, 10, 16)
		if e != nil {
			return o, nil, e
		}
		o.Port = uint(n)
	}
	for k, p := range map[string]*bool{"OFFLINE": &o.Offline, "JSON": &o.JSON, "PARENT_STDIN": &o.ParentStdin} {
		if v := getenv("EP_" + k); v != "" {
			b, e := strconv.ParseBool(v)
			if e != nil {
				return o, nil, e
			}
			*p = b
		}
	}
	fs := flag.NewFlagSet("embedded-postgres", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&config, "config", config, "JSON configuration file")
	fs.StringVar(&o.Version, "postgres-version", o.Version, "pinned PostgreSQL distribution version")
	fs.UintVar(&o.Port, "port", o.Port, "TCP port (0 chooses an available port)")
	fs.StringVar(&o.Database, "database", o.Database, "database name")
	fs.StringVar(&o.Username, "username", o.Username, "database role")
	fs.StringVar(&o.Password, "password", o.Password, "database password (prefer EP_PASSWORD or config)")
	fs.StringVar(&o.CacheDir, "cache-dir", o.CacheDir, "shared binary cache")
	fs.StringVar(&o.WorkDir, "work-dir", o.WorkDir, "parent for disposable workspaces")
	fs.StringVar(&o.DataDir, "data-dir", o.DataDir, "persistent data directory")
	fs.StringVar(&o.Binaries, "binaries", o.Binaries, "existing distribution directory containing bin")
	fs.StringVar(&o.Mirror, "mirror", o.Mirror, "verified download mirror")
	fs.StringVar(&o.User, "user", o.User, "existing Unix UID:GID for PostgreSQL")
	fs.StringVar(&o.SocketDir, "socket-dir", o.SocketDir, "parent for private Unix socket directory")
	fs.StringVar(&o.StartTimeout, "start-timeout", o.StartTimeout, "startup timeout")
	fs.StringVar(&o.StopTimeout, "stop-timeout", o.StopTimeout, "shutdown timeout, maximum 1m")
	fs.StringVar(&o.StateFile, "state-file", o.StateFile, "private file for start/status/stop")
	fs.BoolVar(&o.Offline, "offline", o.Offline, "require cached binaries")
	fs.BoolVar(&o.JSON, "json", o.JSON, "protocol 1 JSON lines on stdout")
	fs.BoolVar(&o.ParentStdin, "parent-stdin", o.ParentStdin, "close server when stdin reaches EOF")
	fs.Func("set", "PostgreSQL setting NAME=VALUE (repeatable)", func(s string) error {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return errors.New("setting needs NAME=VALUE")
		}
		if o.Parameters == nil {
			o.Parameters = map[string]string{}
		}
		o.Parameters[k] = v
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return o, nil, err
	}
	if o.Port > 65535 {
		return o, nil, errors.New("port must be 0..65535")
	}
	return o, fs.Args(), nil
}

func (o Options) config(helper string) (ep.Config, error) {
	start, err := time.ParseDuration(o.StartTimeout)
	if err != nil {
		return ep.Config{}, err
	}
	stop, err := time.ParseDuration(o.StopTimeout)
	if err != nil {
		return ep.Config{}, err
	}
	if o.Password == "" {
		o.Password = rand.Text()
	}
	c := ep.DefaultConfig().Version(ep.PostgresVersion(o.Version)).Port(uint32(o.Port)).Database(o.Database).Username(o.Username).Password(o.Password).Storage(ep.Storage{CacheDir: o.CacheDir, WorkDir: o.WorkDir, DataDir: o.DataDir}).StartTimeout(start).StopTimeout(stop).StartParameters(o.Parameters).Supervisor(helper)
	if o.Binaries != "" {
		c = c.Provider(ep.LocalProvider(o.Binaries))
	} else {
		c = c.Provider(ep.DownloadProvider{CacheDir: o.CacheDir, BaseURL: o.Mirror, Offline: o.Offline})
	}
	if o.SocketDir != "" {
		c = c.UnixSocket(o.SocketDir)
	}
	if o.User != "" {
		u, g, ok := strings.Cut(o.User, ":")
		if !ok {
			return c, errors.New("--user needs UID:GID")
		}
		uid, e := strconv.ParseUint(u, 10, 32)
		if e != nil {
			return c, e
		}
		gid, e := strconv.ParseUint(g, 10, 32)
		if e != nil {
			return c, e
		}
		c = c.RunAs(ep.User{UID: uint32(uid), GID: uint32(gid)})
	}
	return c, nil
}
func (o Options) acquire(ctx context.Context) (*ep.Installation, error) {
	req := ep.BinaryRequest{Version: ep.PostgresVersion(o.Version), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	if o.Binaries != "" {
		return ep.LocalProvider(o.Binaries).Acquire(ctx, req)
	}
	return (ep.DownloadProvider{CacheDir: o.CacheDir, BaseURL: o.Mirror, Offline: o.Offline}).Acquire(ctx, req)
}
