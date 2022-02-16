package main

import (
	"context"
	"encoding/gob"
	"os"

	"go.benjiv.com/sandbox/internal/sig"
)

func init() {
	gob.Register(Info{})
}

type Info struct {
	PID        int
	UID        int
	GID        int
	Terminated bool
}

func main() {
	ctx, cancel := sig.Mon(context.Background())
	defer cancel()

	e := gob.NewEncoder(os.Stdout)

	_ = e.Encode(Info{
		PID: os.Getpid(),
		UID: os.Getuid(),
		GID: os.Getgid(),
	})

	// wait for termination
	<-ctx.Done()
	_ = e.Encode(Info{
		PID:        os.Getpid(),
		UID:        os.Getuid(),
		GID:        os.Getgid(),
		Terminated: true,
	})
}
