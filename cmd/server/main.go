package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"go.benjiv.com/sandbox"
	"go.benjiv.com/sandbox/cmd/internal"
	mytls "go.benjiv.com/sandbox/internal/tls"
	pb "go.benjiv.com/sandbox/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

//go:generate go run ../../tools/certgen/certgen.go -basepath ../../certs

//go:embed roles.json
var config []byte

var timeoutText = `The timeout for releasing a commands resources after the command exits. Valid time units are "ns", "us"
    (or "µs"), "ms", "s", "m", "h".`

func main() {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	caFile := fs.String("ca_file", "../../certs/ca.cert", "The file containing the CA root cert file")
	certFile := fs.String("cert_file", "../../certs/server.cert", "The file containing the CA root cert file")
	keyFile := fs.String("key_file", "../../certs/server.key", "The file containing the CA root cert file")
	serverAddr := fs.String("addr", "127.0.0.1:50000", "The server address in the format of host:port")
	releaseTimeout := fs.Duration("releaseTimeout", time.Minute*5, timeoutText)

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
			var roles mytls.OrgRoles
			err := json.Unmarshal(config, &roles)
			if err != nil {
				return fmt.Errorf("failed to unmarshal roles: %s", err)
			}

			ln, err := net.Listen("tcp", host)
			if err != nil {
				return err
			}
			defer ln.Close()

			opts := []grpc.ServerOption{
				grpc.Creds(credentials.NewTLS(cfg)),
			}

			box, err := sandbox.New(ctx, *releaseTimeout)
			if err != nil {
				return err
			}
			defer func() {
				box.Cleanup()
				lg.Print("cleaned up sandbox")
			}()

			lg.Printf("sandbox created")

			grpcServer := grpc.NewServer(opts...)

			// Initialize the server and register the services.
			cmdSvr, err := pb.NewServer(lg, box, roles)
			if err != nil {
				return err
			}

			pb.RegisterCommandServiceServer(
				grpcServer,
				cmdSvr,
			)

			// Setup a routine to monitor for cancelation and gracefully
			// shutdown the server.
			go func() {
				<-ctx.Done()
				lg.Print("stopping gRPC server")
				defer lg.Print("gRPC server stopped")

				grpcServer.GracefulStop()
			}()

			lg.Printf("serving gRPC server on %s", host)

			return grpcServer.Serve(ln)
		})

	if err != nil {
		os.Exit(1)
	}
}
