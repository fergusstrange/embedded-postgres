package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	ep "github.com/fergusstrange/embedded-postgres/v2"
)

const Protocol = 1

type Event struct {
	Protocol      int    `json:"protocol"`
	Event         string `json:"event"`
	Version       string `json:"version,omitempty"`
	ConnectionURL string `json:"connection_url,omitempty"`
	Port          uint32 `json:"port,omitempty"`
	StateFile     string `json:"state_file,omitempty"`
	Error         string `json:"error,omitempty"`
}

type App struct {
	In         io.Reader
	Out, Err   io.Writer
	Getenv     func(string) string
	Executable string
}

const usage = `embedded-postgres v2: PostgreSQL for tests

Commands:
  run       Run until a signal, control stop, or --parent-stdin EOF
  exec      Run a command with DATABASE_URL; close PostgreSQL on exit
  start     Start in background; requires --state-file
  status    Read an instance through its authenticated local control endpoint
  stop      Close an instance through its authenticated local control endpoint
  prefetch  Download and verify a PostgreSQL distribution
  prune     Remove unused verified PostgreSQL cache entries
  version   Print CLI and protocol versions

Use COMMAND --help for configuration flags. For wrappers:
  embedded-postgres run --json --parent-stdin
  embedded-postgres exec -- npm test
`

func (a App) Main(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprint(a.Out, usage)
		return 0
	}
	if args[0] == "version" {
		_ = json.NewEncoder(a.Out).Encode(Event{Protocol: Protocol, Event: "version", Version: ep.ReleaseVersion})
		return 0
	}
	o, rest, err := parse(args[1:], a.Getenv, a.Err)
	if errors.Is(err, flagHelp) {
		return 0
	}
	code := 1
	if err == nil {
		code, err = a.execute(ctx, args[0], o, rest)
	}
	if err != nil {
		message := err.Error()
		if o.Password != "" {
			message = strings.ReplaceAll(message, o.Password, "[redacted]")
		}
		if o.JSON {
			_ = json.NewEncoder(a.Out).Encode(Event{Protocol: Protocol, Event: "error", Error: message})
		} else {
			fmt.Fprintln(a.Err, message)
		}
		if code == 0 {
			code = 1
		}
	}
	return code
}

func (a App) execute(ctx context.Context, command string, o Options, rest []string) (int, error) {
	if command != "exec" && len(rest) > 0 {
		return 2, errors.New("unexpected positional arguments")
	}
	switch command {
	case "prefetch":
		i, e := o.acquire(ctx)
		if e != nil {
			return 1, e
		}
		if i.Release != nil {
			defer i.Release()
		}
		return 0, json.NewEncoder(a.Out).Encode(map[string]any{"protocol": Protocol, "event": "cached", "directory": i.Dir})
	case "prune":
		count, err := ep.PruneCache(ctx, o.CacheDir)
		if err != nil {
			return 1, err
		}
		return 0, json.NewEncoder(a.Out).Encode(map[string]any{"protocol": Protocol, "event": "pruned", "count": count})
	case "status", "stop":
		event, e := control(ctx, o.StateFile, command)
		if e != nil {
			return 1, e
		}
		return 0, a.emit(o, event)
	case "start":
		return a.start(ctx, o)
	case "run", "exec":
	default:
		return 2, fmt.Errorf("unknown command %q", command)
	}
	if command == "exec" && len(rest) == 0 {
		return 2, errors.New("exec requires a command after --")
	}
	if command == "exec" && o.ParentStdin {
		return 2, errors.New("exec reserves stdin for its child; use run --parent-stdin for wrappers")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The wrapper owns this dedicated pipe. PostgreSQL never inherits it.
	if o.ParentStdin {
		go func() { _, _ = io.Copy(io.Discard, a.In); cancel() }()
	}
	c, err := o.config(a.Executable)
	if err != nil {
		return 2, err
	}
	pg := ep.NewDatabase(c)
	defer pg.Close()
	var ctl *controller
	if o.StateFile != "" {
		ctl, err = newController(ctx, o.StateFile, cancel)
		if err != nil {
			return 1, err
		}
		defer ctl.close()
	}
	if err = pg.StartContext(ctx); err != nil {
		return 1, err
	}
	ready := Event{Protocol: Protocol, Event: "ready", ConnectionURL: pg.ConnectionURL(), Port: pg.GetPort(), StateFile: o.StateFile}
	if ctl != nil {
		if err = ctl.publish(ready); err != nil {
			return 1, err
		}
	}
	if command == "exec" {
		// Keep the test command's stdout/stderr intact; wrappers use run's protocol.
		child := exec.CommandContext(ctx, rest[0], rest[1:]...)
		child.Stdin = a.In
		child.Stdout = a.Out
		child.Stderr = a.Err
		child.Env = childEnvironment(os.Environ(), ready.ConnectionURL)
		child.WaitDelay = 2 * time.Second
		err = child.Run()
		closeErr := pg.Close()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return max(1, exit.ExitCode()), closeErr
			}
			return 1, errors.Join(err, closeErr)
		}
		return 0, closeErr
	}
	if err = a.emit(o, ready); err != nil {
		return 1, err
	}
	select {
	case <-ctx.Done():
	case <-pg.Done():
	}
	err = errors.Join(pg.Close(), pg.Err())
	if err != nil {
		return 1, err
	}
	return 0, a.emit(o, Event{Protocol: Protocol, Event: "stopped"})
}
func (a App) emit(o Options, e Event) error {
	if o.JSON {
		return json.NewEncoder(a.Out).Encode(e)
	}
	if e.ConnectionURL != "" {
		_, err := fmt.Fprintln(a.Out, e.ConnectionURL)
		return err
	}
	_, err := fmt.Fprintln(a.Out, e.Event)
	return err
}
func childEnvironment(env []string, connection string) []string {
	out := []string{}
	for _, v := range env {
		k, _, _ := strings.Cut(v, "=")
		k = strings.ToUpper(k)
		if k == "DATABASE_URL" || strings.HasPrefix(k, "PG") {
			continue
		}
		out = append(out, v)
	}
	u, _ := url.Parse(connection)
	pw, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	if h := u.Query().Get("host"); h != "" {
		host = h
		port = u.Query().Get("port")
	}
	return append(out, "DATABASE_URL="+connection, "PGHOST="+host, "PGPORT="+port, "PGUSER="+u.User.Username(), "PGPASSWORD="+pw, "PGDATABASE="+strings.TrimPrefix(u.Path, "/"), "PGSSLMODE=disable")
}
