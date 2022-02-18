package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.benjiv.com/sandbox/cmd/internal"
	pb "go.benjiv.com/sandbox/proto"
)

func main() {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	caFile := fs.String("ca_file", "../../certs/ca.cert", "The file containing the CA root cert file")
	certFile := fs.String("cert_file", "../../certs/it_admin.cert", "The file containing the CA root cert file")
	keyFile := fs.String("key_file", "../../certs/it_admin.key", "The file containing the CA root cert file")
	serverAddr := fs.String("addr", "127.0.0.1:50000", "The server address in the format of host:port")

	err := internal.Cli(
		fs,
		*caFile,
		*certFile,
		*keyFile,
		*serverAddr,
		func(
			ctx context.Context,
			lg internal.Logger,
			cfg *tls.Config,
			host string,
			args []string,
		) error {
			if len(args) < 3 {
				return fmt.Errorf("missing command")
			}

			conn, client, err := newgRPCClient(ctx, cfg, host)
			if err != nil {
				return fmt.Errorf("failed to create gRPC client: %v", err)
			}
			defer conn.Close()

			c := svcClient{
				client,
				lg,
			}

			fmt.Println(args)

			switch args[1] {
			case "start":
				return c.start(ctx, args[2:])
			case "stop":
				return c.stop(ctx, args[2:])
			case "stat":
				return c.stat(ctx, args[2:])
			case "output":
				return c.output(ctx, args[2:])
			default:
				return internal.ErrFlag
			}
		})

	if err != nil {
		os.Exit(1)
	}
}

type svcClient struct {
	pb.CommandServiceClient
	log internal.Logger
}

func (c svcClient) start(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("start: missing command")
	}

	p, err := c.Start(ctx, &pb.Command{
		Command: args[0],
		Args:    args[1:],
	})

	if err != nil {
		return fmt.Errorf("could not start process: %v", err)
	}

	var a string
	if len(args[1:]) > 0 {
		a = fmt.Sprintf(" with args [%s]", strings.Join(args[1:], " "))
	}

	c.log.Printf(
		"started command [%s]%s; id: %d",
		args[0],
		a,
		p.Id,
	)

	return nil
}

func (c svcClient) stop(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("stop: missing ID")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	var s *pb.Status
	s, err = c.Stop(ctx, &pb.Process{
		Id: int64(id),
	})
	if err != nil {
		return err
	}

	c.log.Print(statusString(int64(id), s))
	return nil
}

func (c svcClient) stat(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("stop: missing ID")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	var s *pb.Status
	s, err = c.Stat(ctx, &pb.Process{
		Id: int64(id),
	})
	if err != nil {
		return fmt.Errorf("could not get status: %v", err)
	}

	c.log.Print(statusString(int64(id), s))
	return nil
}

func (c svcClient) output(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("output: missing ID")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	output, err := c.Output(ctx, &pb.Process{
		Id: int64(id),
	})
	if err != nil {
		return fmt.Errorf("could not get output stream: %v", err)
	}

	var msg *pb.CommandOutput
stream:
	for {
		select {
		case <-ctx.Done():
			break stream
		default:
			msg, err = output.Recv()
			if err != nil {
				break stream
			}

			_, err = os.Stdout.Write(msg.Data)
			if err != nil {
				c.log.Errorf("error while writing to stdout: %s", err)
			}
		}
	}

	if err != nil && err != io.EOF {
		return fmt.Errorf("output stream error: %v", err)
	}

	return nil
}

func newgRPCClient(
	ctx context.Context,
	config *tls.Config,
	serverAddr string,
) (io.Closer, pb.CommandServiceClient, error) {
	// Create a new grpc client connection with the provided
	// TLS credentials.
	conn, err := grpc.DialContext(
		ctx,
		serverAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("did not connect: %v", err)
	}

	return conn, pb.NewCommandServiceClient(conn), nil
}

func statusString(id int64, status *pb.Status) string {
	procStatus := "RUNNING"
	if status.Exited {
		procStatus = fmt.Sprintf(
			"EXITED; exit code: %d",
			int(status.Exitcode),
		)
	}
	return fmt.Sprintf("process %d: %s", id, procStatus)
}
