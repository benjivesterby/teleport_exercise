package sig

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Mon monitors for SIGINT and SIGTERM from the operating system
// and returns a context.Context and a cancel function.
func Mon(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// Setup interrupt monitoring for the agent
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-sigs:
			cancel()
		}
	}()

	return ctx, cancel
}

// Term sends a SIGTERM signal to the command
// to exit gracefully, if that fails then the
// process is killed
func Term(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("cmd is nil")
	}

	err := cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		err := cmd.Process.Signal(os.Kill)
		if err != nil {
			return err
		}
	}
	return nil
}
