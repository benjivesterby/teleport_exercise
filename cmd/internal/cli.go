package internal

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mytls "go.benjiv.com/sandbox/internal/tls"
)

// MainWrap is a type wrap around a custom main function.
// This allows for more control over how context cancellation
// and exit codes are handled.
// It also reduces the amount of boilerplate code needed to
// implement the CLIs since they share the same flags.
type MainWrap func(
	ctx context.Context,
	lg Logger,
	cfg *tls.Config,
	host string,
	args []string,
) error

var ErrFlag = fmt.Errorf("flag error")

// Logger is an interface which is used to log messages.
// TODO: This is exported so both clis can take advantage of it.
type Logger interface {
	Printf(format string, v ...interface{})
	Print(v ...interface{})
	Errorf(format string, v ...interface{})
	Error(v ...interface{})
}

func Cli(
	fs *flag.FlagSet,
	ca, cert, key, host string,
	main MainWrap) (err error) {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// TODO: Swap these for a good logger like `go.devnw.com/alog`
	lg := logger{
		info: log.New(os.Stdout, "[INFO] ", log.LstdFlags),
		err:  log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
	}
	defer func() {
		if err != nil {
			lg.Error(err)
		}
	}()

	help := fs.Bool("h", false, "Show help")
	err = fs.Parse(os.Args)
	if err != nil {
		err = fmt.Errorf("failed to parse flags: %s", err)
		return
	}

	if *help {
		fs.Usage()
		return
	}

	// Collect the remaining arguments.
	args := fs.Args()

	// Load the TLS certificates and create a config.
	var config *tls.Config
	config, err = mytls.LoadConfig(ca, cert, key)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	err = main(ctx, lg, config, host, args)
	if err != nil {
		if err == ErrFlag {
			fs.Usage()
			return nil
		}

		return
	}

	return nil
}

// NOTE: This is copied between the server and client. This should be replaced
// with an actual logger implementation like `go.devnw.com/alog`
type logger struct {
	info *log.Logger
	err  *log.Logger
}

func (l logger) Printf(format string, v ...interface{}) {
	l.info.Printf(format, v...)
}

func (l logger) Print(v ...interface{}) {
	l.info.Println(v...)
}

func (l logger) Error(v ...interface{}) {
	l.err.Println(v...)
}

func (l logger) Errorf(format string, v ...interface{}) {
	l.err.Printf(format, v...)
}
