package sig

import (
	"errors"
	"os/exec"
	"syscall"
)

// Term sends a SIGTERM signal to the command
// to exit gracefully, if that fails then the
// process is killed
func Term(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("cmd is nil")
	}

	err := cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		err := cmd.Process.Kill()
		if err != nil {
			return err
		}
	}
	return nil
}
