package sandbox

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"

	"go.benjiv.com/sandbox/internal/sig"
)

// NOTE: The naming of outPrefix here is to keep it from being
// exported to users of the library since it is not part of the
// public API.
const outPrefix = "stdout-"

// cmdTracker is a wrapper for the exec.Cmd instance
// which handles the creation and execution of the
// subprocess using the helper process.
type cmdTracker struct {
	id       int
	ctx      context.Context
	cancel   context.CancelFunc
	command  string
	args     []string
	cmd      *exec.Cmd
	stdout   string
	status   chan Status
	output   chan io.ReadCloser
	stop     chan struct{}
	finished chan int
}

// cmdInfo is a type enforced wrapper for the
// channels on the cmdTracker to ensure that
// consumers are unable to modify the channels
// in inappropriate ways.
type cmdInfo struct {
	id       int
	stop     chan<- struct{}
	status   <-chan Status
	output   <-chan io.ReadCloser
	finished <-chan int
}

// Create a new command instance using the helper binary
// and the provided arguments.
//
// This function creates the underlying mapping for the stdout
// and stderr of the
func createCmd(
	ctx context.Context,
	tempdir string,
	helper string, // path to helper process
	command string,
	args ...string,
) (cmdInfo, error) {
	// TODO: This is not a valid solution for a production
	// system due to the potential for collisions. This should
	// be changed to a UUID instead to ensure the risk of a
	// collision is mitigated.
	id, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return cmdInfo{}, err
	}

	// setup the the wrappers internal context
	// and cancel functions.
	ctx, cancel := context.WithCancel(ctx)

	outputFile := filepath.Join(
		tempdir,
		fmt.Sprintf("%s%d", outPrefix, id),
	)

	// Initialize the helper command with
	// the proper arguments.
	cmd, err := creatHelperCmd(
		helper,
		outputFile,
		command,
		args...,
	)
	if err != nil {
		cancel()
		return cmdInfo{}, err
	}

	err = cmd.Start()
	if err != nil {
		cancel()
		return cmdInfo{}, err
	}

	finished := make(chan int)
	go func() {
		defer close(finished)

		// NOTE: I am purposely ignoring this
		// error since stderr and stdout are
		// already being merged and the exit
		// status is being checked.
		err = cmd.Wait()
		exitcode := 0

		if exitError, ok := err.(*exec.ExitError); ok {
			exitcode = exitError.ExitCode()
		}

		for {
			select {
			case <-ctx.Done():
				return
			case finished <- exitcode:
			}
		}
	}()

	// Create the command tracker instance
	// and start the internal goroutine for
	// managing data access and the command
	return (&cmdTracker{
		id:       int(id.Int64()),
		ctx:      ctx,
		cancel:   cancel,
		command:  command,
		args:     args,
		cmd:      cmd,
		stdout:   outputFile,
		status:   make(chan Status),
		output:   make(chan io.ReadCloser),
		stop:     make(chan struct{}),
		finished: finished,
	}).init()
}

// init starts an routine for managing and acessing the
// command instance
func (c *cmdTracker) init() (cmdInfo, error) {
	go func() {
		exited := false
		exitcode := 0
		finished := c.finished

		for {
			select {
			case <-c.ctx.Done():
				// NOTE: I am purposely ignoring this
				// error as it is not critical to the
				// operation of the command.
				_ = sig.Term(c.cmd)
				return
			case <-c.stop:
				if exited {
					continue
				}

				// NOTE: I am purposely ignoring this
				// error as it is not critical to the
				// operation of the command.
				_ = sig.Term(c.cmd)
			case exitCode, ok := <-finished:
				if !ok {
					return
				}

				exited = true
				exitcode = exitCode

				// nil out the channel so this select
				// statement will continue to read
				// from the finished channel.
				finished = nil
			case c.status <- Status{
				Exited: exited,
				Code:   exitcode,
			}:
			case c.output <- c.reader(c.stdout):
			}
		}
	}()

	// Return a wrapper which limits the access to the
	// channels to ensure that consumers cannot modify
	// the cmdTracker in an improper manner.
	return cmdInfo{
		id:       c.id,
		status:   c.status,
		output:   c.output,
		stop:     c.stop,
		finished: c.finished,
	}, nil
}

// reader opens a read only file and returns a file wrapper
// which handles the EOF condition.
func (c *cmdTracker) reader(file string) io.ReadCloser {
	r, err := os.OpenFile(file, os.O_RDONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &fileWrapper{r, c.finished}
}

// createHelperCmd creates a new command instance for the
// helper process, merges the stdout and stderr of the
// command, and returns the command instance.
func creatHelperCmd(
	path string,
	stdout string,
	command string,
	args ...string,
) (*exec.Cmd, error) {
	// Create the command instance for the helper
	// proc which will handle the subprocess
	// creation and execution.
	// TODO: I'm not terribly keen on the hard coding
	// for `run` here. Ideally the command would be pre-built
	// and all this function would do is add the arguments and
	// stdout/stderr files to the command.
	// nolint:gosec
	cmd := exec.Command(
		path,
		append(
			[]string{"run", command},
			args...,
		)...,
	)

	// Create a stdout file for the output of the
	// subprocess.
	writer, err := os.OpenFile(
		stdout,
		os.O_CREATE|os.O_WRONLY,
		os.ModeNamedPipe,
	)
	if err != nil {
		return nil, err
	}

	// merge the stdout and stderr
	cmd.Stdout = writer
	cmd.Stderr = cmd.Stdout

	return cmd, nil
}

// fileWrapper wraps a *os.File and adds a channel which is
// closed when the command has finished.
type fileWrapper struct {
	*os.File
	finished <-chan int
}

// Read overrides the underlying Read method to check if the
// command has finished. In the event that the reader has
// reached an EOF and the command is finished, it will return
// ErrCommandFinished.
func (f *fileWrapper) Read(p []byte) (n int, err error) {
	n, err = f.File.Read(p)

	// If the command has finished and the reader returned
	// end of file then return the ErrCommandFinished to tell
	// the caller that the command has finished and the reader
	// should be closed.
	select {
	case <-f.finished:
		if err == io.EOF {
			return n, err
		}
	default:
	}

	// Override the EOF error because the command
	// is not complete.
	if err == io.EOF {
		err = nil
	}

	return n, err
}
