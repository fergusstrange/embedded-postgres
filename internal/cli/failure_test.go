package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOptionsFilesEnvironmentAndIdentity(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config")
	for _, contents := range []string{`{"port":1} {}`, `{"port":1} trailing`, `{"port":"wrong"}`, `{`} {
		os.WriteFile(file, []byte(contents), 0600)
		if _, _, e := parse([]string{"--config=" + file}, func(string) string { return "" }, io.Discard); e == nil {
			t.Fatal("invalid config accepted", contents)
		}
	}
	os.WriteFile(file, []byte(`{"port":1234}`), 0600)
	if o, _, e := parse([]string{"-config=" + file}, func(string) string { return "" }, io.Discard); e != nil || o.Port != 1234 {
		t.Fatal(o, e)
	}
	env := map[string]string{"EP_CONFIG": file, "EP_PORT": "2345", "EP_OFFLINE": "true", "EP_JSON": "true", "EP_PARENT_STDIN": "true", "EP_USERNAME": "custom", "EP_POSTGRES_VERSION": "17.11.0"}
	o, _, e := parse(nil, func(k string) string { return env[k] }, io.Discard)
	if e != nil || o.Port != 2345 || !o.JSON || !o.ParentStdin || !o.Offline || o.Username != "custom" {
		t.Fatal(o, e)
	}
	env["EP_JSON"] = "bad"
	if _, _, e = parse(nil, func(k string) string { return env[k] }, io.Discard); e == nil {
		t.Fatal("invalid boolean")
	}
	base, _, e := parse(nil, func(string) string { return "" }, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []Options{func() Options { x := base; x.StartTimeout = "bad"; return x }(), func() Options { x := base; x.StopTimeout = "bad"; return x }(), func() Options { x := base; x.User = "missing-colon"; return x }(), func() Options { x := base; x.User = "bad:1"; return x }(), func() Options { x := base; x.User = "1:bad"; return x }()} {
		if _, e = o.config("helper"); e == nil {
			t.Fatal("bad config accepted")
		}
	}
	base.User = "10001:10001"
	base.SocketDir = "/socket"
	base.Password = "chosen"
	base.Binaries = t.TempDir()
	if _, e = base.config("helper"); e != nil {
		t.Fatal(e)
	}
	if _, e = base.acquire(t.Context()); e == nil {
		t.Fatal("invalid local distribution")
	}
	base.Binaries = ""
	base.CacheDir = t.TempDir()
	base.Offline = true
	if _, e = base.acquire(t.Context()); e == nil {
		t.Fatal("offline cache miss")
	}
}
func TestControlFailuresAndStaleState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	if _, e := readState(""); e == nil {
		t.Fatal("empty state path")
	}
	if _, e := readState(path); e == nil {
		t.Fatal("missing state")
	}
	os.WriteFile(path, []byte("user file"), 0600)
	if c, e := newController(t.Context(), path, func() {}); e == nil {
		c.close()
		t.Fatal("overwrote user file")
	}
	if b, _ := os.ReadFile(path); string(b) != "user file" {
		t.Fatal("user file changed")
	}
	if c, e := newController(t.Context(), filepath.Join(path, "child"), func() {}); e == nil {
		c.close()
		t.Fatal("invalid parent")
	}
	for _, kind := range []string{"unauthorized", "invalid-json", "wrong-protocol", "stop", "timeout", "unavailable"} {
		t.Run(kind, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "state")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "unauthorized":
					w.WriteHeader(401)
				case "invalid-json":
					io.WriteString(w, "broken")
				case "wrong-protocol":
					io.WriteString(w, `{"protocol":99}`)
				default:
					json.NewEncoder(w).Encode(Event{Protocol: Protocol, Event: "stopping"})
					if kind == "stop" {
						os.Remove(statePath)
					}
				}
			}))
			defer srv.Close()
			b, _ := json.Marshal(controlState{Protocol: Protocol, URL: srv.URL, Token: strings.Repeat("x", 32)})
			os.WriteFile(statePath, b, 0600)
			if kind == "unavailable" {
				srv.Close()
			}
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			command := "status"
			if kind == "stop" || kind == "timeout" {
				command = "stop"
			}
			event, e := control(ctx, statePath, command)
			if kind == "stop" {
				if e != nil || event.Event != "stopped" {
					t.Fatal(event, e)
				}
			} else if e == nil {
				t.Fatal("invalid control response accepted")
			}
		})
	}
}
func TestControlStartupAndPublishingErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c, e := newController(ctx, path, cancel)
	if e != nil {
		t.Fatal(e)
	}
	defer c.close()
	request, _ := http.NewRequest("GET", c.state.URL+"/status", nil)
	request.Header.Set("Authorization", "Bearer "+c.state.Token)
	response, e := http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatal("reported ready before publication")
	}
	request.URL.Path = "/unknown"
	response, e = http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("unknown route")
	}
	original := c.path
	c.path = filepath.Join(path, "missing", "file")
	if e = c.publish(Event{Protocol: Protocol, Event: "ready"}); e == nil {
		t.Fatal("invalid publication path")
	}
	c.path = original
	os.Mkdir(path, 0700)
	os.WriteFile(filepath.Join(path, "user-data"), nil, 0600)
	if e = c.publish(Event{Protocol: Protocol, Event: "ready"}); e == nil {
		t.Fatal("removed nonempty state directory")
	}
}
func TestCLIStartupFailures(t *testing.T) {
	var out bytes.Buffer
	a := App{In: strings.NewReader(""), Out: &out, Err: io.Discard, Getenv: func(string) string { return "" }, Executable: "missing-helper"}
	cases := [][]string{{"run", "--binaries", t.TempDir(), "--json"}, {"run", "--start-timeout", "bad"}, {"exec", "--parent-stdin", "--", "child"}, {"start", "--parent-stdin", "--state-file", filepath.Join(t.TempDir(), "state")}, {"start", "--state-file", filepath.Join(t.TempDir(), "state")}, {"prefetch", "--offline", "--cache-dir", t.TempDir()}, {"status", "--state-file", "missing"}}
	for _, args := range cases {
		if a.Main(t.Context(), args) == 0 {
			t.Error("invalid command succeeded", args)
		}
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0600)
	if _, e := a.start(t.Context(), Options{StateFile: filepath.Join(file, "state")}); e == nil {
		t.Fatal("invalid state parent")
	}
	path := filepath.Join(t.TempDir(), "state")
	os.Mkdir(path+".log", 0700)
	if _, e := a.start(t.Context(), Options{StateFile: path}); e == nil {
		t.Fatal("invalid log path")
	}
	if a.Main(t.Context(), []string{"prune", "--cache-dir", t.TempDir()}) != 0 {
		t.Fatal("empty cache prune failed")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestCLIConsumerDisconnect(t *testing.T) {
	bin, helper := cliFixture(t)
	parent := t.TempDir()
	a := App{In: strings.NewReader(""), Out: brokenWriter{}, Err: io.Discard, Getenv: os.Getenv, Executable: helper}
	o, _, e := parse([]string{"--binaries", bin, "--work-dir", parent, "--json"}, os.Getenv, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.execute(t.Context(), "run", o, nil); !errors.Is(e, io.ErrClosedPipe) {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(parent)
	if e != nil || len(entries) != 0 {
		t.Fatal("disconnected consumer retained work", e)
	}
}
func TestCLIExecSuccessAndMissingCommand(t *testing.T) {
	bin, helper := cliFixture(t)
	var out bytes.Buffer
	a := App{In: strings.NewReader(""), Out: &out, Err: io.Discard, Getenv: os.Getenv, Executable: helper}
	o, _, e := parse([]string{"--binaries", bin}, os.Getenv, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	if code, e := a.execute(t.Context(), "exec", o, []string{helper, "version"}); e != nil || code != 0 {
		t.Fatal(code, e)
	}
	if code, e := a.execute(t.Context(), "exec", o, []string{"no-such-test-command"}); e == nil || code != 1 {
		t.Fatal(code, e)
	}
}
