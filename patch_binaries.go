package embeddedpostgres

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func patchBinaries(extractPath string, interpreterPath string) error {
	if interpreterPath == "" {
		return nil
	}

	binaries := [3]string{"pg_ctl", "initdb", "postgres"}
	for _, binary := range binaries {
		path := filepath.Join(extractPath, "bin", binary)
		_, fileErr := os.Stat(path)
		if os.IsNotExist(fileErr) {
			continue
		}
		if err := exec.Command("patchelf", "--set-interpreter", interpreterPath, path).Run(); err != nil {
			return fmt.Errorf("unable to patch %s: %s", binary, err)
		}
	}

	return nil
}
