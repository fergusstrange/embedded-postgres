// Package embeddedpostgres runs an isolated PostgreSQL server for tests.
// Production packages use only the Go standard library. PostgreSQL and the
// companion supervisor are native executables acquired separately.
package embeddedpostgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fergusstrange/embedded-postgres/v2/internal/filelock"
	"github.com/fergusstrange/embedded-postgres/v2/internal/platform"
	"github.com/fergusstrange/embedded-postgres/v2/internal/supervisor"
)

var (
	ErrServerNotStarted     = errors.New("server has not been started")
	ErrServerAlreadyStarted = errors.New("server is already started")
	ErrDataVersion          = errors.New("persistent data belongs to a different PostgreSQL major version")
)

// EmbeddedPostgres serialises lifecycle transitions. Separate instances run in
// parallel and may share immutable binaries. Do not copy after first use.
type EmbeddedPostgres struct {
	mu       sync.Mutex
	config   Config
	active   *session
	lastInfo InstanceInfo
	lastErr  error
	lastLog  string
}
type session struct {
	supervisorPath    string
	postgresPID       int
	info              InstanceInfo
	installation      *Installation
	lock              *os.File
	cmd               *exec.Cmd
	lease             io.WriteCloser
	exited            chan struct{}
	waitErr           error
	closed            chan struct{}
	cleanups          []func(context.Context) error
	logPath, passPath string
}

// NewDatabase constructs an instance without doing I/O. If supplied, the first
// config is used, preserving the v1 calling convention.
func NewDatabase(config ...Config) *EmbeddedPostgres {
	c := DefaultConfig()
	if len(config) > 0 {
		c = config[0]
	}
	c.startParameters = maps.Clone(c.startParameters)
	c.environment = append([]string(nil), c.environment...)
	c = c.Hooks(c.hooks)
	return &EmbeddedPostgres{config: c}
}
func (ep *EmbeddedPostgres) Start() error { return ep.StartContext(context.Background()) }

// StartContext starts a server whose lifetime is bound to ctx. Cancellation
// after readiness also closes the server, using an independent cleanup timeout.
func (ep *EmbeddedPostgres) StartContext(ctx context.Context) (err error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != nil {
		return ErrServerAlreadyStarted
	}
	c := ep.config
	if err = validateConfig(c); err != nil {
		return err
	}
	if err = platform.ValidateIdentity(c.identity); err != nil {
		return err
	}
	start, cancel := context.WithTimeout(ctx, c.startTimeout)
	defer cancel()
	s := &session{closed: make(chan struct{})}
	ep.active = s
	ep.lastErr = nil
	ep.lastLog = ""
	defer func() {
		if p := recover(); p != nil {
			_ = ep.closeLocked(context.Background())
			panic(p)
		}
		if err != nil {
			err = errors.Join(err, ep.closeLocked(context.Background()))
			ep.lastErr = err
		}
	}()
	parent := c.storage.WorkDir
	if parent == "" && c.identity != nil {
		parent = "/tmp"
	}
	if parent != "" {
		if err = os.MkdirAll(parent, 0755); err != nil {
			return err
		}
	}
	work, err := os.MkdirTemp(parent, "embedded-postgres-")
	if err != nil {
		return err
	}
	work, err = filepath.Abs(work)
	if err != nil {
		return err
	}
	s.info.WorkDir = work
	s.logPath = filepath.Join(work, "postgres.log")
	s.passPath = filepath.Join(work, "pgpass")
	if err = platform.Own(work, c.identity); err != nil {
		return err
	}
	if err = ownedFile(s.logPath, nil, c.identity); err != nil {
		return err
	}
	provider := c.provider
	if provider == nil {
		if c.binariesPath != "" {
			provider = LocalProvider(c.binariesPath)
		} else {
			provider = DownloadProvider{CacheDir: c.storage.CacheDir, BaseURL: c.repositoryURL}
		}
	}
	s.installation, err = provider.Acquire(start, BinaryRequest{c.version, runtime.GOOS, runtime.GOARCH})
	if err != nil {
		return fmt.Errorf("acquire PostgreSQL: %w", err)
	}
	if s.installation == nil {
		return errors.New("provider returned nil installation")
	}
	if err = validateInstallation(s.installation.Dir); err != nil {
		return err
	}
	s.info.BinariesDir, err = filepath.Abs(s.installation.Dir)
	if err != nil {
		return err
	}
	if c.identity != nil {
		copied := filepath.Join(work, "installation")
		if err = copyInstallation(start, s.info.BinariesDir, copied); err != nil {
			return err
		}
		s.info.BinariesDir = copied
	}
	helper, err := resolveSupervisor(start, c)
	if err != nil {
		return err
	}
	if c.identity != nil {
		dest := filepath.Join(work, executable("supervisor"))
		if err = copyFile(helper, dest, 0755); err != nil {
			return err
		}
		helper = dest
	}
	s.supervisorPath = helper
	version, err := ep.command(start, s, "postgres", "--version")
	if err != nil {
		return fmt.Errorf("PostgreSQL runtime unavailable (check native libraries and RunAs path access): %w", err)
	}
	major := strings.Split(string(c.version), ".")[0]
	if !strings.Contains(string(version), " "+major+".") {
		return fmt.Errorf("provider returned unexpected PostgreSQL version: %s", strings.TrimSpace(string(version)))
	}
	data := c.storage.DataDir
	if data == "" {
		data = filepath.Join(work, "data")
	} else {
		data, err = filepath.Abs(data)
		if err != nil {
			return err
		}
	}
	s.info.DataDir = data
	if err = os.MkdirAll(filepath.Dir(data), 0755); err != nil {
		return err
	}
	s.lock, err = filelock.Acquire(start, data+".embedded-postgres.lock", true)
	if err != nil {
		return fmt.Errorf("lock data directory: %w", err)
	}
	fresh := false
	if st, e := os.Lstat(data); os.IsNotExist(e) {
		if err = os.Mkdir(data, 0700); err != nil {
			return err
		}
		if err = platform.Own(data, c.identity); err != nil {
			return err
		}
		fresh = true
	} else if e != nil {
		return e
	} else if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("data directory must be a real directory, not a symlink")
	}
	if !fresh {
		entries, e := os.ReadDir(data)
		if e != nil {
			return e
		}
		fresh = len(entries) == 0
		if !fresh {
			v, e := os.ReadFile(filepath.Join(data, "PG_VERSION"))
			if e != nil {
				return fmt.Errorf("data directory is nonempty and not a PostgreSQL cluster: %w", e)
			}
			if strings.TrimSpace(string(v)) != major {
				return fmt.Errorf("%w: found %s, requested %s", ErrDataVersion, strings.TrimSpace(string(v)), major)
			}
		}
	}
	if fresh {
		pw := filepath.Join(work, "init-password")
		if err = ownedFile(pw, []byte(c.password+"\n"), c.identity); err != nil {
			return err
		}
		_, err = ep.command(start, s, "initdb", "-D", data, "-U", c.username, "--auth=scram-sha-256", "--pwfile="+pw, "--locale="+c.locale, "--encoding="+c.encoding)
		removeErr := os.Remove(pw)
		if err = errors.Join(err, removeErr); err != nil {
			return fmt.Errorf("initialise PostgreSQL: %w", err)
		}
	}
	if c.socketDir != "" {
		if runtime.GOOS == "windows" {
			return errors.New("Unix sockets are not supported on Windows")
		}
		socketDir, e := filepath.Abs(c.socketDir)
		if e != nil {
			return e
		}
		if err = os.MkdirAll(socketDir, 0700); err != nil {
			return err
		}
		// The parent is caller-owned; only the private socket child is removed.
		socketDir, err = os.MkdirTemp(socketDir, "ep-")
		if err != nil {
			return err
		}
		if err = platform.Own(socketDir, c.identity); err != nil {
			return err
		}
		s.cleanups = append(s.cleanups, func(context.Context) error { return os.RemoveAll(socketDir) })
		c.socketDir = socketDir
	}
	for _, hook := range c.hooks.BeforeStart {
		cleanup, e := hook(start, s.info)
		if cleanup != nil {
			s.cleanups = append(s.cleanups, cleanup)
		}
		if e != nil {
			return fmt.Errorf("before-start hook: %w", e)
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		port := c.port
		if port == 0 {
			port, err = freePort()
			if err != nil {
				return err
			}
		}
		s.info.Port = port
		s.info.ConnectionURL = connectionURL(c, port)
		if err = ownedFile(s.passPath, []byte("*:*:*:"+pgpass(c.username)+":"+pgpass(c.password)+"\n"), c.identity); err != nil {
			return err
		}
		args := []string{"-D", data, "-p", fmt.Sprint(port), "-h", "127.0.0.1", "-c", "unix_socket_directories="}
		if c.socketDir != "" {
			args = []string{"-D", data, "-p", fmt.Sprint(port), "-h", "", "-c", "unix_socket_directories=" + c.socketDir}
		}
		keys := make([]string, 0, len(c.startParameters))
		for k := range c.startParameters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-c", k+"="+c.startParameters[k])
		}
		err = ep.launch(start, s, helper, args)
		if err == nil {
			err = ep.ready(start, s, c)
		}
		if err == nil {
			break
		}
		_ = ep.stopProcess(s)
		logs := logTail(s.logPath)
		if c.port != 0 || c.socketDir != "" || (!strings.Contains(string(logs), "Address already in use") && !strings.Contains(string(logs), "could not bind")) {
			return err
		}
		if err = os.WriteFile(s.logPath, nil, 0600); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	if fresh && c.database != "postgres" {
		if _, err = ep.command(start, s, "createdb", "-w", "--maintenance-db="+withoutPassword(connectionURL(cWithDatabase(c, "postgres"), s.info.Port)), "--", c.database); err != nil {
			return fmt.Errorf("create database: %w", err)
		}
	}
	if _, err = ep.command(start, s, "psql", "-X", "-w", "-v", "ON_ERROR_STOP=1", "-d", withoutPassword(s.info.ConnectionURL), "-Atc", "SELECT 1"); err != nil {
		return err
	}
	for _, hook := range c.hooks.Ready {
		cleanup, e := hook(start, s.info)
		if cleanup != nil {
			s.cleanups = append(s.cleanups, cleanup)
		}
		if e != nil {
			return fmt.Errorf("ready hook: %w", e)
		}
	}
	ep.lastInfo = s.info
	go func() {
		select {
		case <-ctx.Done():
			_ = ep.closeSession(s)
		case <-s.exited:
			_ = ep.closeSession(s)
		case <-s.closed:
		}
	}()
	return nil
}
func (ep *EmbeddedPostgres) launch(ctx context.Context, s *session, helper string, args []string) error {
	cmd, release, err := platform.Command(context.Background(), ep.config.identity, helper, "__supervise")
	if err != nil {
		return err
	}
	defer release()
	cmd.Dir = s.info.WorkDir
	cmd.Env = ep.environment(s)
	lease, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		lease.Close()
		return err
	}
	log, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		lease.Close()
		output.Close()
		return err
	}
	defer log.Close()
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		lease.Close()
		output.Close()
		return err
	}
	s.cmd = cmd
	s.lease = lease
	s.exited = make(chan struct{})
	s.waitErr = nil
	go func() { s.waitErr = cmd.Wait(); close(s.exited) }()
	req := supervisor.Request{Protocol: supervisor.Protocol, Binary: filepath.Join(s.info.BinariesDir, "bin", executable("postgres")), Args: args, DataDir: s.info.DataDir, LogPath: s.logPath, ShutdownTimeout: ep.config.stopTimeout}
	if err = json.NewEncoder(lease).Encode(req); err != nil {
		return err
	}
	type handshake struct {
		event supervisor.Event
		err   error
	}
	event := make(chan handshake, 1)
	go func() {
		defer output.Close()
		var e supervisor.Event
		err := json.NewDecoder(output).Decode(&e)
		if err == nil && (e.Protocol != supervisor.Protocol || e.PID <= 0) {
			err = errors.New("invalid supervisor handshake")
		}
		event <- handshake{e, err}
	}()
	select {
	case result := <-event:
		s.postgresPID = result.event.PID
		return result.err
	case <-ctx.Done():
		lease.Close()
		return ctx.Err()
	}
}
func (ep *EmbeddedPostgres) ready(ctx context.Context, s *session, c Config) error {
	for {
		select {
		case <-s.exited:
			return ep.failure(s, errors.New("supervisor exited during startup"))
		default:
		}
		probe, cancel := context.WithTimeout(ctx, time.Second)
		_, err := ep.command(probe, s, "psql", "-X", "-w", "-d", withoutPassword(connectionURL(cWithDatabase(c, "postgres"), s.info.Port)), "-Atc", "SELECT 1")
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ep.failure(s, errors.Join(ctx.Err(), err))
		case <-s.exited:
			return ep.failure(s, err)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func (ep *EmbeddedPostgres) command(ctx context.Context, s *session, name string, args ...string) ([]byte, error) {
	cmd, release, err := platform.Command(ctx, ep.config.identity, s.supervisorPath, "__command")
	if err != nil {
		return nil, err
	}
	defer release()
	cmd.Dir = s.info.WorkDir
	cmd.Env = ep.environment(s)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer input.Close()
	cmd.Cancel = func() error { return input.Close() }
	cmd.WaitDelay = ep.config.stopTimeout + 2*time.Second
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	req := supervisor.Request{Protocol: supervisor.Protocol, Binary: filepath.Join(s.info.BinariesDir, "bin", executable(name)), Args: args, DataDir: s.info.WorkDir}
	if err = json.NewEncoder(input).Encode(req); err != nil {
		input.Close()
		_ = cmd.Wait()
		return nil, err
	}
	err = cmd.Wait()
	if err != nil {
		return output.Bytes(), fmt.Errorf("%s: %w: %s", name, err, redact(output.String(), ep.config.password))
	}
	return output.Bytes(), nil
}

func (ep *EmbeddedPostgres) environment(s *session) []string {
	env := []string{}
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "PG") || key == "HOME" || key == "TMPDIR" {
			continue
		}
		env = append(env, v)
	}
	env = append(env, ep.config.environment...)
	return append(env, "HOME="+s.info.WorkDir, "TMPDIR="+s.info.WorkDir, "PGPASSFILE="+s.passPath, "PGCONNECT_TIMEOUT=1")
}

// Stop preserves the v1 error when no server is active. Close is idempotent.
func (ep *EmbeddedPostgres) Stop() error {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active == nil {
		return ErrServerNotStarted
	}
	return ep.closeLocked(context.Background())
}

// StopContext requests cleanup and bounds how long the caller waits. If ctx
// expires, bounded process shutdown and resource cleanup continue independently.
func (ep *EmbeddedPostgres) StopContext(ctx context.Context) error {
	ep.mu.Lock()
	s := ep.active
	ep.mu.Unlock()
	if s == nil {
		return ErrServerNotStarted
	}
	done := make(chan error, 1)
	go func() { done <- ep.closeSession(s) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (ep *EmbeddedPostgres) Close() error {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.closeLocked(context.Background())
}
func (ep *EmbeddedPostgres) stopProcess(s *session) error {
	if s.lease == nil {
		return nil
	}
	_ = s.lease.Close()
	select {
	case <-s.exited:
	case <-time.After(ep.config.stopTimeout + 2*time.Second):
		if s.postgresPID > 0 {
			if p, e := os.FindProcess(s.postgresPID); e == nil {
				_ = platform.Kill(p)
				_ = p.Release()
			}
		}
		_ = platform.Kill(s.cmd.Process)
		<-s.exited
	}
	s.lease = nil
	if s.waitErr != nil {
		return ep.failure(s, s.waitErr)
	}
	return nil
}
func (ep *EmbeddedPostgres) closeLocked(_ context.Context) (err error) {
	s := ep.active
	if s == nil {
		return nil
	}
	defer func() { ep.lastInfo = s.info; ep.active = nil; close(s.closed); ep.lastErr = err }()
	err = ep.stopProcess(s)
	cleanup, cancel := context.WithTimeout(context.Background(), ep.config.stopTimeout)
	defer cancel()
	var firstPanic any
	call := func(fn func() error) (e error) {
		defer func() {
			if p := recover(); p != nil {
				if firstPanic == nil {
					firstPanic = p
				}
				e = fmt.Errorf("cleanup panicked: %v", p)
			}
		}()
		return fn()
	}
	defer func() {
		if firstPanic != nil {
			panic(firstPanic)
		}
	}()
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		err = errors.Join(err, call(func() error { return s.cleanups[i](cleanup) }))
	}
	ep.lastLog = redact(string(logTail(s.logPath)), ep.config.password)
	if ep.config.logger != nil {
		if b := ep.lastLog; len(b) > 0 {
			_, e := io.WriteString(ep.config.logger, redact(string(b), ep.config.password))
			err = errors.Join(err, e)
		}
	}
	if s.lock != nil {
		err = errors.Join(err, s.lock.Close())
	}
	if s.info.WorkDir != "" {
		err = errors.Join(err, os.RemoveAll(s.info.WorkDir))
	}
	if s.installation != nil && s.installation.Release != nil {
		err = errors.Join(err, call(s.installation.Release))
	}
	return err
}

// ConnectionURL returns the actual endpoint after Start succeeds.
func (ep *EmbeddedPostgres) ConnectionURL() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != nil {
		return ep.active.info.ConnectionURL
	}
	return ep.lastInfo.ConnectionURL
}
func (ep *EmbeddedPostgres) GetConnectionURL() string { return ep.ConnectionURL() }
func (ep *EmbeddedPostgres) GetPort() uint32 {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != nil {
		return ep.active.info.Port
	}
	return ep.lastInfo.Port
}
func (ep *EmbeddedPostgres) Info() InstanceInfo {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != nil {
		return ep.active.info
	}
	return ep.lastInfo
}
func (ep *EmbeddedPostgres) failure(s *session, err error) error {
	b := logTail(s.logPath)
	return fmt.Errorf("%w\n%s", err, redact(string(b), ep.config.password))
}
func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := uint32(l.Addr().(*net.TCPAddr).Port)
	return port, l.Close()
}
func ownedFile(path string, b []byte, id *User) error {
	if err := os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	return platform.Own(path, id)
}
func pgpass(s string) string { return strings.NewReplacer("\\", "\\\\", ":", "\\:").Replace(s) }
func redact(s, password string) string {
	if password != "" {
		s = strings.ReplaceAll(s, password, "[redacted]")
	}
	return s
}
func cWithDatabase(c Config, d string) Config { c.database = d; return c }
func validateConfig(c Config) error {
	if c.port > 65535 {
		return errors.New("port must be 0..65535")
	}
	if c.startTimeout <= 0 || c.stopTimeout <= 0 || c.stopTimeout > time.Minute {
		return errors.New("positive startup and shutdown timeouts required; shutdown maximum is one minute")
	}
	parts := strings.Split(string(c.version), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return errors.New("invalid PostgreSQL version")
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 10, 32); err != nil {
			return errors.New("invalid PostgreSQL version")
		}
	}
	for _, s := range []string{c.database, c.username, c.password, c.locale, c.encoding} {
		if s == "" || strings.ContainsAny(s, "\x00\r\n") {
			return errors.New("database, user, password, locale and encoding must be nonempty and contain no NUL or newline")
		}
	}
	for k, v := range c.startParameters {
		switch strings.ToLower(k) {
		case "port", "listen_addresses", "data_directory", "unix_socket_directories", "config_file", "hba_file", "ident_file", "external_pid_file":
			return fmt.Errorf("%s is managed by the lifecycle; use its dedicated setting or a hook", k)
		}
		if k == "" || strings.ContainsAny(k, "=\x00\r\n") || strings.ContainsRune(v, 0) {
			return errors.New("invalid PostgreSQL setting")
		}
	}
	if c.provider != nil && (c.binariesPath != "" || c.repositoryURL != "") {
		return errors.New("Provider cannot be combined with BinariesPath or BinaryRepositoryURL")
	}
	return nil
}

// Done closes when an instance has been fully cleaned up. Before Start it is already closed.
func (ep *EmbeddedPostgres) Done() <-chan struct{} {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != nil {
		return ep.active.closed
	}
	done := make(chan struct{})
	close(done)
	return done
}

// Err reports the most recent lifecycle or unexpected-process error.
func (ep *EmbeddedPostgres) Err() error { ep.mu.Lock(); defer ep.mu.Unlock(); return ep.lastErr }

// Logs returns the recent server output with the configured password redacted.
// A bounded tail remains available after Close for test-failure diagnostics.
// Temporary files are still removed. Errors also include startup diagnostics.
func (ep *EmbeddedPostgres) Logs() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active == nil {
		return ep.lastLog
	}
	b := logTail(ep.active.logPath)
	return redact(string(b), ep.config.password)
}

// logTail bounds memory even when a test produces large server logs.
func logTail(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	if st.Size() > 32<<10 {
		if _, err = f.Seek(-(32 << 10), io.SeekEnd); err != nil {
			return nil
		}
	}
	b, _ := io.ReadAll(io.LimitReader(f, 32<<10))
	return b
}

// A delayed watcher from an old lifetime must never close a restarted instance.
func (ep *EmbeddedPostgres) closeSession(s *session) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("cleanup panicked: %v", p)
		}
	}()
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.active != s {
		return nil
	}
	return ep.closeLocked(context.Background())
}
