package embeddedpostgres

import (
	"fmt"
	"os/exec"
)

var (
	errNotSupported = fmt.Errorf("RunAsUser config parameter not supported on windows")
)

func setRunAs(process *exec.Cmd, runAsUser string) error {
	return errNotSupported
}

func chown(file string, runAsUser string) error {
	return errNotSupported
}
