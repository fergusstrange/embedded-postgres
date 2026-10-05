package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fergusstrange/embedded-postgres/v2/internal/filelock"
)

type controlState struct {
	Protocol int    `json:"protocol"`
	URL      string `json:"url"`
	Token    string `json:"token"`
}
type controller struct {
	path     string
	state    controlState
	lock     *os.File
	listener net.Listener
	server   *http.Server
	mu       sync.RWMutex
	event    Event
}

func newController(ctx context.Context, path string, cancel context.CancelFunc) (*controller, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := filelock.Try(path + ".lock")
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		lock.Close()
		return nil, err
	}
	c := &controller{path: path, lock: lock, listener: listener, state: controlState{Protocol: Protocol, URL: "http://" + listener.Addr().String(), Token: rand.Text()}}
	c.server = &http.Server{ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 4096}
	c.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+c.state.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/stop" {
			cancel()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Event{Protocol: Protocol, Event: "stopping"})
			return
		}
		if r.Method != "GET" || r.URL.Path != "/status" {
			http.NotFound(w, r)
			return
		}
		c.mu.RLock()
		event := c.event
		c.mu.RUnlock()
		if event.Event == "" {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(event)
	})
	go func() { _ = c.server.Serve(listener) }()
	return c, nil
}
func (c *controller) publish(event Event) error {
	c.mu.Lock()
	c.event = event
	c.mu.Unlock()
	f, err := os.CreateTemp(filepath.Dir(c.path), ".ep-state-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(c.state)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	// Remove only the explicitly selected state file, under its exclusive lease.
	if err = os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(f.Name(), c.path)
}
func (c *controller) close() {
	_ = c.server.Close()
	_ = os.Remove(c.path)
	_ = c.lock.Close()
}
func readState(path string) (controlState, error) {
	var s controlState
	if path == "" {
		return s, errors.New("--state-file is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	if err = json.NewDecoder(io.LimitReader(f, 4096)).Decode(&s); err != nil {
		return s, err
	}
	u, err := url.Parse(s.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || s.Protocol != Protocol || len(s.Token) < 20 {
		return s, errors.New("invalid local control state")
	}
	return s, nil
}
func control(ctx context.Context, path, command string) (Event, error) {
	s, err := readState(path)
	if err != nil {
		return Event{}, err
	}
	method := "GET"
	if command == "stop" {
		method = "POST"
	}
	req, err := http.NewRequestWithContext(ctx, method, s.URL+"/"+command, nil)
	if err != nil {
		return Event{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	client := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return Event{}, fmt.Errorf("instance unavailable; state may be stale: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Event{}, fmt.Errorf("control HTTP %d", res.StatusCode)
	}
	var event Event
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&event); err != nil {
		return event, err
	}
	if command == "stop" {
		deadline, cancel := context.WithTimeout(ctx, 70*time.Second)
		defer cancel()
		for {
			current, e := readState(path)
			if os.IsNotExist(e) || (e == nil && current.Token != s.Token) {
				return Event{Protocol: Protocol, Event: "stopped"}, nil
			}
			select {
			case <-deadline.Done():
				return Event{}, deadline.Err()
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	return event, nil
}
func (a App) start(ctx context.Context, o Options) (int, error) {
	if o.StateFile == "" {
		return 2, errors.New("start requires --state-file")
	}
	if o.ParentStdin {
		return 2, errors.New("start cannot use --parent-stdin; use run for a parent-bound server")
	}
	state, err := filepath.Abs(o.StateFile)
	if err != nil {
		return 1, err
	}
	o.StateFile = state
	if err = os.MkdirAll(filepath.Dir(state), 0700); err != nil {
		return 1, err
	}
	// The child reads a private config once; no credentials go in its argv.
	f, err := os.CreateTemp(filepath.Dir(state), ".ep-config-")
	if err != nil {
		return 1, err
	}
	defer os.Remove(f.Name())
	childOptions := o
	childOptions.JSON = true
	err = json.NewEncoder(f).Encode(childOptions)
	err = errors.Join(err, f.Close())
	if err != nil {
		return 1, err
	}
	log, err := os.OpenFile(state+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 1, err
	}
	defer log.Close()
	child := exec.Command(a.Executable, "run", "--config", f.Name())
	// Prevent the launcher's environment from overriding the serialized config.
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(v), "EP_") {
			child.Env = append(child.Env, v)
		}
	}
	child.Stderr = log
	detach(child)
	output, err := child.StdoutPipe()
	if err != nil {
		return 1, err
	}
	defer output.Close()
	if err = child.Start(); err != nil {
		return 1, err
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	success := false
	defer func() {
		if !success {
			_ = child.Process.Kill()
			<-done
		}
	}()
	type result struct {
		event Event
		err   error
	}
	ready := make(chan result, 1)
	go func() {
		var e Event
		err := json.NewDecoder(io.LimitReader(output, 64<<10)).Decode(&e)
		ready <- result{e, err}
	}()
	timeout, err := time.ParseDuration(o.StartTimeout)
	if err != nil {
		return 2, err
	}
	wait, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	select {
	case r := <-ready:
		if r.err != nil {
			return 1, fmt.Errorf("background startup: %w (see %s.log)", r.err, state)
		}
		if r.event.Event != "ready" {
			return 1, errors.New(r.event.Error)
		}
		success = true
		return 0, a.emit(o, r.event)
	case <-wait.Done():
		return 1, wait.Err()
	}
}
