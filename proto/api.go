package proto

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.benjiv.com/sandbox"
	"go.benjiv.com/sandbox/internal/tls"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// logger is an interface which is used to log messages.
type logger interface {
	Printf(format string, v ...interface{})
	Print(v ...interface{})
	Errorf(format string, v ...interface{})
	Error(v ...interface{})
}

// cmdSrv is a server implementation of the CommandServiceServer interface.
type cmdSrv struct {
	UnimplementedCommandServiceServer
	box   *sandbox.Box
	roles tls.OrgRoles
	log   logger
}

// ErrAuthenticationFailure is the default error returned by the server
// regardless of any other errors. This is to ensure that no information is
// leaked to the client.
var ErrAuthenticationFailure = fmt.Errorf("authentication failure")

// Start verifies the roles embedded in the certificate and starts the process.
func (c *cmdSrv) Start(ctx context.Context, in *Command) (*Process, error) {
	cert, err := c.certFromContext(ctx)
	if err != nil {
		c.log.Errorf("failed to get cert from context: %s", err)
		return nil, ErrAuthenticationFailure
	}

	err = c.roleCheck(in.Command, cert)
	if err != nil {
		// TODO: These logs should be higher than "ERROR" and should
		// trigger notifications to the security team as they are
		// potentially a security issue.
		c.log.Errorf(
			"cert [%d] failed role check for command %s: %s",
			int(cert.SerialNumber.Int64()),
			in.Command,
			err,
		)
		return nil, ErrAuthenticationFailure
	}

	id, err := c.box.Start(in.Command, in.Args...)
	if err != nil {
		c.log.Errorf("failed to start process: %s", err)
		return nil, err
	}

	var args string
	if len(in.Args) > 0 {
		args = fmt.Sprintf(" with args [%s]", strings.Join(in.Args, " "))
	}

	c.log.Printf(
		"starting command [%s]%s for cert [%d]; id: %d",
		in.Command,
		args,
		int(cert.SerialNumber.Int64()),
		id,
	)
	return &Process{
		Id: int64(id),
	}, nil
}

// Output streams the stdout and stderr of the process to the client.
func (c *cmdSrv) Output(in *Process, svc CommandService_OutputServer) error {
	id, err := c.roleCheckByID(svc.Context(), in.Id)
	if err != nil {
		// TODO: These logs should be higher than "ERROR" and should
		// trigger notifications to the security team as they are
		// potentially a security issue.
		c.log.Errorf(
			"cert [%d] failed role check for process %d: %s",
			id,
			in.Id,
			err,
		)
		return ErrAuthenticationFailure
	}

	rc, err := c.box.Output(int(in.Id))
	if err != nil {
		c.log.Errorf("failed to get output: %s", err)
		return err
	}
	defer rc.Close()

	c.log.Printf(
		"streaming output of process %d for cert [%d]",
		in.Id,
		id,
	)
	buff := make([]byte, 1024)
	for {
		// Adhere to the context.
		select {
		case <-svc.Context().Done():
			return svc.Context().Err()
		default:
		}

		n, err := rc.Read(buff)
		if err == io.EOF {
			break
		}

		if err != nil {
			c.log.Errorf("error reading output for process %d: %s", in.Id, err)
			return err
		}

		err = svc.Send(&CommandOutput{
			Data: buff[:n],
		})
		if err != nil {
			c.log.Errorf("error sending output for process %d: %s", in.Id, err)
			return err
		}
	}

	return nil
}

func (c *cmdSrv) Stop(ctx context.Context, in *Process) (*Status, error) {
	id, err := c.roleCheckByID(ctx, in.Id)
	if err != nil {
		// TODO: These logs should be higher than "ERROR" and should
		// trigger notifications to the security team as they are
		// potentially a security issue.
		c.log.Errorf(
			"cert [%d] failed role check for process %d: %s",
			id,
			in.Id,
			err,
		)
		return nil, ErrAuthenticationFailure
	}

	c.log.Printf("stopping process %d for certificate %d", in.Id, id)
	err = c.box.Stop(int(in.Id))
	if err != nil {
		c.log.Errorf("failed to stop process: %s", err)
		return nil, err
	}

	return c.Stat(ctx, in)
}

// Stat returns the status of the command.
func (c *cmdSrv) Stat(ctx context.Context, in *Process) (*Status, error) {
	status, err := c.box.Stat(int(in.Id))
	if err != nil {
		return nil, errors.New("process not found")
	}

	cert, err := c.certFromContext(ctx)
	if err != nil {
		return nil, ErrAuthenticationFailure
	}

	err = c.roleCheck(status.Command, cert)
	if err != nil {
		// TODO: These logs should be higher than "ERROR" and should
		// trigger notifications to the security team as they are
		// potentially a security issue.
		c.log.Errorf(
			"cert [%d] failed role check for process %d: %s",
			int(cert.SerialNumber.Int64()),
			in.Id,
			err,
		)
		return nil, ErrAuthenticationFailure
	}

	c.log.Printf(
		"stating process %d for certificate %d",
		in.Id,
		int(cert.SerialNumber.Int64()),
	)
	return &Status{
		Exited:   status.Exited,
		Exitcode: int32(status.Code),
	}, nil
}

// NewServer creates a new instances of the CmdSrv server which adds the
// implementation of the CommandServiceServer interface by shadowing the
// methods of the UnimplementedCommandServiceServer interface which is
// embedded in the CmdSrv struct.
func NewServer(
	log logger,
	box *sandbox.Box,
	roles tls.OrgRoles,
) (CommandServiceServer, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}

	return &cmdSrv{
		box:   box,
		roles: roles,
		log:   log,
	}, nil
}

// roleCheckByID stat's the process and checks the role of the certificate
// against the command that is running with that id. If the command for that
// id is NOT allowed to run by the certificate, an error is returned, otherwise
// nil.
func (c *cmdSrv) roleCheckByID(ctx context.Context, id int64) (int, error) {
	status, err := c.box.Stat(int(id))
	if err != nil {
		return 0, errors.New("process not found")
	}

	cert, err := c.certFromContext(ctx)
	if err != nil {
		return 0, ErrAuthenticationFailure
	}

	err = c.roleCheck(status.Command, cert)
	if err != nil {
		return 0, ErrAuthenticationFailure
	}

	return int(cert.SerialNumber.Int64()), nil
}

// certFromContext extracts the TLS certificate from the context using the
// gRPC peer information.
func (c *cmdSrv) certFromContext(
	ctx context.Context,
) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, ErrAuthenticationFailure
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, ErrAuthenticationFailure
	}

	// TODO: This is purposely limiting the number of certificates to one.
	// Ideally in a production system multiple certificates could be included
	// and the system would properly negotiate roles across the certificates.
	if len(tlsInfo.State.PeerCertificates) != 1 {
		return nil, ErrAuthenticationFailure
	}

	return tlsInfo.State.PeerCertificates[0], nil
}

// roleCheck handles the role evaluation for the given command using the
// metadata from the supplied certificate and the roles defined in the server.
// If the command is not allowed for the roleset an error is returned, otherwise,
// the command is allowed.
func (c *cmdSrv) roleCheck(
	command string,
	cert *x509.Certificate,
) error {
	// negotiate allowed commands for this set of orgs and units.
	commands := tls.GetCommands(
		c.roles,
		cert.Subject.Organization,
		cert.Subject.OrganizationalUnit,
	)

	// Bypass the role check if the user has the "admin" role.
	// TODO: This is a very simple, insecure implementation and should NOT
	// be used in production.
	if _, full := commands["*"]; !full {
		// Ensure that the commands list for this user allows the command.
		if canExec, has := commands[command]; !has || !canExec {
			return ErrAuthenticationFailure
		}
	}

	return nil
}
