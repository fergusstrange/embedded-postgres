package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if e := os.WriteFile(path, []byte(`{"port":1234,"database":"file","json":true}`), 0600); e != nil {
		t.Fatal(e)
	}
	o, rest, e := parse([]string{"--config", path, "--database", "flag", "--set", "max_connections=30", "--", "child"}, func(k string) string {
		if k == "EP_DATABASE" {
			return "env"
		}
		if k == "EP_PORT" {
			return "2345"
		}
		return ""
	}, io.Discard)
	if e != nil || o.Database != "flag" || o.Port != 2345 || !o.JSON || o.Parameters["max_connections"] != "30" || len(rest) != 1 {
		t.Fatalf("%+v %v %v", o, rest, e)
	}
}
func TestInvalidOptions(t *testing.T) {
	for _, args := range [][]string{{"--config"}, {"--port", "65536"}, {"--port", "-1"}, {"--set", "broken"}, {"--unknown"}, {"--config", "missing"}} {
		if _, _, e := parse(args, func(string) string { return "" }, io.Discard); e == nil {
			t.Errorf("accepted %v", args)
		}
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte(`{"surprise":true}`), 0600)
	if _, _, e := parse([]string{"--config", path}, func(string) string { return "" }, io.Discard); e == nil {
		t.Fatal("unknown JSON field")
	}
	for _, v := range []string{"bad", "-1"} {
		if _, _, e := parse(nil, func(k string) string {
			if k == "EP_PORT" {
				return v
			}
			return ""
		}, io.Discard); e == nil {
			t.Fatal("bad env port")
		}
	}
}
func TestMainCommands(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		code   int
		output string
	}{{nil, 0, "Commands:"}, {[]string{"version"}, 0, `"protocol":1`}, {[]string{"unknown", "--json"}, 2, `"event":"error"`}, {[]string{"run", "--help"}, 0, ""}, {[]string{"exec"}, 2, ""}, {[]string{"start"}, 2, ""}, {[]string{"status"}, 1, ""}, {[]string{"run", "extra"}, 2, ""}} {
		var out, stderr bytes.Buffer
		a := App{In: strings.NewReader(""), Out: &out, Err: &stderr, Getenv: func(string) string { return "" }}
		if c := a.Main(t.Context(), tc.args); c != tc.code || !strings.Contains(out.String(), tc.output) {
			t.Errorf("%v: %d %s %s", tc.args, c, &out, &stderr)
		}
	}
}
func TestChildEnvironment(t *testing.T) {
	env := childEnvironment([]string{"PGHOST=wrong", "DATABASE_URL=wrong", "PATH=keep"}, "postgresql://user:p%40ss@127.0.0.1:1234/my%20db?sslmode=disable")
	got := strings.Join(env, "\n")
	for _, want := range []string{"PGHOST=127.0.0.1", "PGPORT=1234", "PGPASSWORD=p@ss", "PGDATABASE=my db", "PATH=keep"} {
		if !strings.Contains(got, want) {
			t.Error(got)
		}
	}
	if strings.Contains(got, "wrong") {
		t.Fatal(got)
	}
}
func TestControlAuthentication(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	path := filepath.Join(t.TempDir(), "state.json")
	c, e := newController(ctx, path, cancel)
	if e != nil {
		t.Fatal(e)
	}
	defer c.close()
	if e = c.publish(Event{Protocol: Protocol, Event: "ready", Port: 1234}); e != nil {
		t.Fatal(e)
	}
	response, e := http.Get(c.state.URL + "/status")
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("unauthenticated request succeeded")
	}
	event, e := control(ctx, path, "status")
	if e != nil || event.Port != 1234 {
		t.Fatal(event, e)
	}
	if other, e := newController(ctx, path, cancel); e == nil {
		other.close()
		t.Fatal("duplicate controller")
	}
	request, _ := http.NewRequest("POST", c.state.URL+"/stop", nil)
	request.Header.Set("Authorization", "Bearer "+c.state.Token)
	response, e = http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel")
	}
}
func TestRejectRemoteControlState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	for _, u := range []string{"https://127.0.0.1:1234", "http://example.com:1234", "http://127.0.0.1:1234/redirect", "http://user@127.0.0.1:1234"} {
		b, _ := json.Marshal(controlState{Protocol: Protocol, URL: u, Token: strings.Repeat("x", 32)})
		os.WriteFile(path, b, 0600)
		if _, e := readState(path); e == nil {
			t.Error("accepted", u)
		}
	}
}
func cliFixture(t *testing.T) (string, string) {
	t.Helper()
	bin, helper := os.Getenv("EP_TEST_BIN"), os.Getenv("EP_SUPERVISOR")
	if bin == "" || helper == "" {
		t.Skip("set EP_TEST_BIN and EP_SUPERVISOR")
	}
	return filepath.Dir(bin), helper
}
func TestCLIParentPipe(t *testing.T) {
	bin, helper := cliFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, "run", "--binaries", bin, "--json", "--parent-stdin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	output, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		input.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	dec := json.NewDecoder(output)
	var ready Event
	if e = dec.Decode(&ready); e != nil || ready.Event != "ready" || ready.ConnectionURL == "" {
		t.Fatalf("%+v %v %s", ready, e, &stderr)
	}
	input.Close()
	var stopped Event
	if e = dec.Decode(&stopped); e != nil || stopped.Event != "stopped" {
		t.Fatal(stopped, e, &stderr)
	}
	if e = cmd.Wait(); e != nil {
		t.Fatal(e, &stderr)
	}
}
func TestCLIBackground(t *testing.T) {
	bin, helper := cliFixture(t)
	state := filepath.Join(t.TempDir(), "instance.json")
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	args := []string{"start", "--state-file", state, "--binaries", bin, "--json"}
	output, e := exec.CommandContext(ctx, helper, args...).CombinedOutput()
	if e != nil {
		t.Fatalf("%s %v", output, e)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		control(cleanup, state, "stop")
	})
	ready, e := control(ctx, state, "status")
	if e != nil || ready.Port == 0 {
		t.Fatal(ready, e)
	}
	stopped, e := control(ctx, state, "stop")
	if e != nil || stopped.Event != "stopped" {
		t.Fatal(stopped, e)
	}
	if _, e = os.Stat(state); !os.IsNotExist(e) {
		t.Fatal("state retained", e)
	}
}
func TestCLIExec(t *testing.T) {
	bin, helper := cliFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	// The test executable validates the child's environment and exit propagation.
	cmd := exec.CommandContext(ctx, helper, "exec", "--binaries", bin, "--", os.Args[0], "-test.run=^TestCLIExecChild$")
	cmd.Env = append(os.Environ(), "EP_CHILD_TEST=1")
	output, e := cmd.CombinedOutput()
	var exit *exec.ExitError
	if e == nil || !errors.As(e, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("%s %v", output, e)
	}
}
func TestCLIExecChild(t *testing.T) {
	if os.Getenv("EP_CHILD_TEST") != "1" {
		return
	}
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("PGPORT") == "0" {
		os.Exit(9)
	}
	os.Exit(7)
}
