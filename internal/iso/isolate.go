package iso

import (
	"os"
	"os/exec"
	"syscall"
)

// Isolate creates an *exec.Cmd with isolation syscall flags for execution and
// maps the command's stdin, stdout, and stderr to that of the parent process.
//
// TODO: This function has a hardcoded call to `sub` as an argument to the
// currently running command. This is not correct for a production system and
// should be made configurable in the future.
// TODO: It also assumes the position of arguments is correct. This is likely
// not the case for a production system.
func Isolate() *exec.Cmd {
	// nolint:gosec
	cmd := exec.Command(os.Args[0], append([]string{"sub"}, os.Args[2:]...)...)

	// Pipe in the parent's stdin and pipe out the child's stdout and stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Setup the isolation settings
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	return cmd
}
