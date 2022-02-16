package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.benjiv.com/sandbox/internal/cgroups"
	"go.benjiv.com/sandbox/internal/iso"
	"go.benjiv.com/sandbox/internal/sig"
)

// NOTE: I am arbitrarily using an exit code of 2 for helper
// process errors. This will likely have a collision with
// other exit codes in the future. This should be updated
// to indicate to the caller specific errors and be sufficiently
// unique to avoid collisions. I am also NOT checking the exit
// status in the caller in an attempt to deal with errors in the
// subprocess. This is not correct for a production system and
// would need to be resolved.

func usage() {
	fmt.Println("Usage: prochelper run|sub <cmd> <args>")
}

//nolint:gocritic
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// Setup signal monitoring
	ctx, cancel := sig.Mon(context.Background())
	defer cancel()

	var cmd *exec.Cmd
	switch os.Args[1] {
	case "run": // Isolate the process
		cmd = iso.Isolate()

		data, err := os.ReadFile("constraints.json")
		if err == nil {
			err = cgroups.LimitResources("phelper", data)
			if err != nil {
				cancel()
				os.Exit(2)
			}
		}
	case "sub": // Run the command provided as an argument
		// nolint:gosec
		cmd = exec.Command(os.Args[2], os.Args[3:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	default:
		usage()
		os.Exit(1)
	}

	// Initiate the command
	err := cmd.Start()
	if err != nil {
		cancel()
		os.Exit(2)
	}

	// Cascade sigterm to the child processes
	// and kill if the sigterm fails
	go func() {
		<-ctx.Done()

		// Send terminate / kill
		err = sig.Term(cmd)
		if err != nil {
			os.Exit(2)
		}
	}()

	// Wait for the child process to exit
	// Wait populates the ProcessState
	err = cmd.Wait()
	if exitError, ok := err.(*exec.ExitError); ok {
		os.Exit(exitError.ExitCode())
	}

	os.Exit(0)
}
