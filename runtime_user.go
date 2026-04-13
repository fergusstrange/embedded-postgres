package embeddedpostgres

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const freebsdRuntimeUser = "embeddedpg"

func shouldUseFreeBSDRuntimeUser() bool {
	return runtime.GOOS == "freebsd" && os.Geteuid() == 0
}

func ensureFreeBSDRuntimeUser(binaryExtractLocation, runtimePath, dataPath string) error {
	if !shouldUseFreeBSDRuntimeUser() {
		return nil
	}

	if err := exec.Command("pw", "groupshow", freebsdRuntimeUser).Run(); err != nil {
		cmd := exec.Command("pw", "useradd", freebsdRuntimeUser, "-m", "-s", "/bin/sh")
		if output, addErr := cmd.CombinedOutput(); addErr != nil {
			return errorWithOutput(addErr, output)
		}
	}

	for _, path := range []string{runtimePath, filepath.Dir(dataPath), dataPath} {
		if path == "" {
			continue
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	for _, path := range []string{binaryExtractLocation, runtimePath, dataPath, filepath.Dir(dataPath)} {
		if path == "" {
			continue
		}
		cmd := exec.Command("chown", "-R", freebsdRuntimeUser+":"+freebsdRuntimeUser, path)
		if output, chownErr := cmd.CombinedOutput(); chownErr != nil {
			return errorWithOutput(chownErr, output)
		}
	}

	return nil
}

func wrapCommandForRuntimeUser(cmd *exec.Cmd) *exec.Cmd {
	if !shouldUseFreeBSDRuntimeUser() {
		return cmd
	}

	args := append([]string{"-m", freebsdRuntimeUser, "-c", shellQuoteCommand(cmd.Path, cmd.Args[1:])}, []string{}...)
	wrapped := exec.Command("su", args...)
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	wrapped.Env = cmd.Env
	wrapped.Dir = cmd.Dir
	return wrapped
}

func shellQuoteCommand(binary string, args []string) string {
	quoted := shellQuote(binary)
	for _, arg := range args {
		quoted += " " + shellQuote(arg)
	}
	return quoted
}

func shellQuote(value string) string {
	escaped := "'"
	for _, r := range value {
		if r == '\'' {
			escaped += "'\"'\"'"
		} else {
			escaped += string(r)
		}
	}
	escaped += "'"
	return escaped
}

func errorWithOutput(err error, output []byte) error {
	if len(output) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, string(output))
}
