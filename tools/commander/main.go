package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.benjiv.com/sandbox"
	"go.benjiv.com/sandbox/internal/sig"
)

const (
	ERROR = 1
	OK    = 0
)

func main() {
	exitcode := OK
	defer func() {
		os.Exit(exitcode)
	}()

	ctx, cancel := sig.Mon(context.Background())
	defer cancel()

	box, err := sandbox.New(ctx)
	if err != nil {
		panic(err)
	}
	defer box.Cleanup()

	if len(os.Args) < 1 {
		fmt.Println("Usage: commander <cmd> <args>")
		exitcode = ERROR
		return
	}

	id, err := box.Start(os.Args[1], os.Args[2:]...)
	if err != nil {
		fmt.Println(err)
		exitcode = ERROR
		return
	}

	go func(id int) {
		<-ctx.Done()
		err = box.Stop(id)
		if err != nil && err != context.Canceled {
			fmt.Println(err)
			exitcode = ERROR
		}
	}(id)

	output, err := box.Output(id)
	if err != nil {
		fmt.Println(err)
		exitcode = ERROR
		return
	}
	defer output.Close()

	buf := make([]byte, 1024)
	for {
		n, err := output.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			exitcode = ERROR
			return
		}
		fmt.Print(string(buf[:n]))
	}
}
