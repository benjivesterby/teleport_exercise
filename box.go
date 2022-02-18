package sandbox

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// New returns a sandbox environment after creating the parent
// cgroups which manages the processes within the library.
//
// The releaseTimeout `time.Duration` is the amount of time the
// sandbox environment will retain a process's information,
// after process execution completes, in memory and on disk
// before it's released.
func New(ctx context.Context, releaseTimeout time.Duration) (*Box, error) {
	tempDir, helper, err := deployHelper()
	if err != nil {
		return nil, err
	}

	return &Box{
		ctx:            ctx,
		tempDir:        tempDir,
		helperPath:     helper,
		catalog:        make(map[int]cmdInfo),
		releaseTimeout: releaseTimeout,
	}, nil
}

// Box manages an internal collection of processes and resources.
// Each instance of Box has its own isolated cgroup and is
// responsible for creating subprocesses using the helper binary.
type Box struct {
	ctx            context.Context
	tempDir        string
	helperPath     string
	catalog        map[int]cmdInfo
	catalogMu      sync.RWMutex
	releaseTimeout time.Duration
}

// Cleanup will remove the sandbox temp directory
// and all of its contents.
func (b *Box) Cleanup() {
	var boxWg sync.WaitGroup
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	// Add all processes to the wait group
	boxWg.Add(len(b.catalog))
	for _, info := range b.catalog {
		go func(info cmdInfo) {
			defer boxWg.Done()
			for {
				select {
				case info.stop <- struct{}{}:
				case s, ok := <-info.status:
					if !ok || s.Exited {
						return
					}
				}
			}
		}(info)
	}

	// Wait for all processes to exit
	boxWg.Wait()

	// Cleanup the temp directory
	os.RemoveAll(b.tempDir)
}

// Start executes the commands in the sandbox environment.
func (b *Box) Start(cmd string, args ...string) (id int, err error) {
	// Create and execute the command passing in
	// the context, temp directory, and the helper binary path.
	info, err := createCmd(
		b.tempDir,
		b.helperPath,
		b.releaseTimeout,
		cmd,
		args...,
	)
	if err != nil {
		return 0, err
	}

	// Lock and catalog the process
	b.catalogMu.Lock()
	defer b.catalogMu.Unlock()

	// Add the new process cmdInfo to the catalog
	// of running processes.
	b.catalog[info.id] = info

	return info.id, nil
}

// Stop will cancel the child context used to call the helper binary, the helper
// binary will monitor for sigterm and will cancel the subprocess context.
func (b *Box) Stop(id int) error {
	// Load the process info from the catalog
	info, err := b.getInfo(id)
	if err != nil {
		return err
	}

	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case status, ok := <-info.status:
		if !ok {
			b.rmProc(id)
			return ErrProcessNotFound
		}

		if !status.Exited {
			// Stop the process
			// NOTE: I am NOT closing the stop channel here.
			// This is to ensure the select in the cmdTracker
			// is not constantly taking the `<-info.stop` case
			// statement since the `select` statement is
			// stochastic in its execution.
			info.stop <- struct{}{}
		}
	}

	return nil
}

// Status indicates the current status of the process and if
// the process has exited the exit code will be included
type Status struct {
	Command string
	Exited  bool
	Code    int
}

// Stat returns the status of the process with the given id.
func (b *Box) Stat(id int) (Status, error) {
	// Load the process info from the catalog
	info, err := b.getInfo(id)
	if err != nil {
		return Status{}, err
	}

	// Get the status of the process from the process info
	select {
	case <-b.ctx.Done():
		return Status{}, b.ctx.Err()
	case status, ok := <-info.status:
		if !ok {
			b.rmProc(id)
			return Status{}, ErrProcessNotFound
		}

		return status, nil
	}
}

// Output returns a io.ReadCloser instance for reading the
// output of the process for the given id.
func (b *Box) Output(id int) (io.ReadCloser, error) {
	// Load the process info from the catalog
	info, err := b.getInfo(id)
	if err != nil {
		return nil, err
	}

	// Pull the output io.ReadCloser from the process info
	select {
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	case output, ok := <-info.output:
		if !ok {
			b.rmProc(id)
			return nil, ErrProcessNotFound
		}

		return output, nil
	}
}

var ErrProcessNotFound = errors.New("process not found")

// rmProc removes the process with the given id from the catalog.
func (b *Box) rmProc(id int) {
	// Lock the catalog
	b.catalogMu.Lock()
	defer b.catalogMu.Unlock()

	// Remove the process from the catalog
	delete(b.catalog, id)
}

// getInfo will return the cmdInfo for the given id.
// This method is split out to allow for testing as well
// as minimizing the total lock time.
func (b *Box) getInfo(id int) (cmdInfo, error) {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	i, ok := b.catalog[id]
	if !ok {
		return cmdInfo{}, ErrProcessNotFound
	}

	return i, nil
}
