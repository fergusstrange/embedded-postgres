//go:build !linux && !darwin && !windows && !freebsd

package platform

import (
	"errors"
	"os"
	"os/exec"
)

func ValidateIdentity(*Identity) error               { return errors.New("unsupported operating system") }
func configure(*exec.Cmd, *Identity) (func(), error) { return nil, ValidateIdentity(nil) }
func Kill(p *os.Process) error                       { return p.Kill() }
func Own(string, *Identity) error                    { return ValidateIdentity(nil) }
func Guard() error                                   { return ValidateIdentity(nil) }
