//go:build !windows || !(amd64 || arm64)

package servers

import (
	"errors"
	"os"
	"os/exec"
)

func quiet(*exec.Cmd) {}

func contain(cmd *exec.Cmd) (kill func() error, err error) {
	return func() error {
		if err := cmd.Process.Kill(); !errors.Is(err, os.ErrProcessDone) {
			return err
		}

		return nil
	}, nil
}
