package embeddedpostgres

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type InitDatabase func(binaryExtractLocation, username, password string) error

func defaultInitDatabase(binaryExtractLocation, username, password string) error {
	passwordFile, err := createPasswordFile(binaryExtractLocation, password)
	if err != nil {
		return err
	}
	postgresInitDbBinary := filepath.Join(binaryExtractLocation, "bin/initdb")
	postgresInitDbProcess := exec.Command(postgresInitDbBinary,
		"-A", "password",
		"-U", username,
		"-D", filepath.Join(binaryExtractLocation, "data"),
		fmt.Sprintf("--pwfile=%s", passwordFile))
	postgresInitDbProcess.Stderr = os.Stderr
	postgresInitDbProcess.Stdout = os.Stdout
	if err := postgresInitDbProcess.Run(); err != nil {
		return fmt.Errorf("unable to init database using: %s", postgresInitDbProcess.String())
	}
	return nil
}

func createPasswordFile(binaryExtractLocation, password string) (string, error) {
	passwordFileLocation := filepath.Join(binaryExtractLocation, "pwfile")
	if err := ioutil.WriteFile(passwordFileLocation, []byte(password), 0600); err != nil {
		return "", fmt.Errorf("unable to write password file to %s", passwordFileLocation)
	}
	return passwordFileLocation, nil
}
