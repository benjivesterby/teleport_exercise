# Design Document

## Styling

For styling I will use the linter styling rules enforced by my golangci-lint
config (`.golangci.yml`) which are opinionated, but consistent. For non-linter
supported styling I use the [Go style guide](https://github.com/golang/go/wiki/CodeReviewComments).

## Expected Process Flow

![Process Flow Diagram](diagrams/processflow.svg)

## Worker Library

This library will be responsible for encapsulating the necessary syscalls for
isolation and deployment of (and attachment to) cgroups for resource control.
The library will also embed a helper binary for running subprocesses.

When a client executes a start request, the library will generate a random ID
and create cgroup folders for that ID under the parent PID. The cgroup files
will then be updated with the resource constraints, and the helper process PID
will be written to the cgroups.

### cgroup Configuration

- `notify_on_release` in `pids` to clean up the cgroup folder on exit
- `memory.limit_in_bytes` set to `209715200` (200MB)
- `cpu.cfs_quota_us` set to `10000` (10ms or 1/10 of the default 100ms set in
  `cpu.cfs_period_us`)
- `blkio.throttle.write_bps_device` set to `8:0 10485760` (10MB/s for primary
  disk)

### syscall Configuration

- `syscall.CLONE_NEWUTS` for hostname and NIS domain name isolation
- `syscall.CLONE_NEWPID` for process isolation
- `syscall.CLONE_NEWNS` for mounting isolation
- `syscall.CLONE_NEWNET` for network isolation

**NOTE:** I am not configuring network connections for sub-processes in the
exercise. No commands will have network access.

### Helper Binary

This helper binary will be responsible for running the subprocess in isolation
and for controlling the subprocess's resources. This helper binary will utilize
environment variables to configure the resource limits and will passthrough the
command and arguments to the subprocess.

### Exported API

The API will be simple, abstracting away the complicated details of isolation
and resource constraints. Each "E" (environment) variable can have any number of
isolated processes with their own resource limits.

```go
// Env will return an environment after creating the parent cgroups which
// manages the processes within the library.
func Env(context.Context) E

// E is the Environment type. I chose E as the name to reduce the overall
// length of the name (and avoid duplicative naming). This can always be
// updated to be more verbose, but the only exported method that will
// supply it is the `Env` function which documents that it returns
// an environment.
type E struct {
    // ... Unexported Fields
}

func (e *E) Start(cmd string, args...string) (id int, err error)
func (e *E) Stop(id int) error
func (e *E) Stat(id int) (*os.ProcessState, error)
func (e *E) Output(id int) (io.Reader, error)
```

**TRADEOFF:** For simplicity I have chosen to merge the stdout and stderr into a
single stream as the "output" of the command. This is not ideal for a production
instance as it doesn't allow for differentiation between the two streams.

**TRADEOFF:** In an effort to preserve the existing system `$PATH` execution
environment I have chosen not to remap the root of the isolated process. This
will allow the client to have full access to the binaries on the system for
execution.

**TRADEOFF:** Usually when developing a library it is preferred that the user is
able supply configurations such as resource control and network isolation
values. This implementation will hard code these values in the library itself to
simplify the API for the client.

**TRADEOFF:** In general, unbounded parallelism is not a good idea, but, with
the limited (non-production) scope, I have chosen to use the `go` primitive
without protections. This is less of a concern for this design due to the
resource control of the implementation.

### Testing

Library testing will follow a similar test helper process pattern to the
`os/exec` package itself. The testing will be minimal to account for time. The
helper process will be tested to ensure proper propagation of the environment
variables and stdin/stdout/stderr.

## API

The API is responsible for accepting a command and arguments, and executing the
specific method action in the library. For example, a call to the `Start`
endpoint will start a new isolated and constrained instance of the command
(using the library) with the provided arguments.

The `protobuf` definition for the gRPC service and the messages are located in
the [proto/api.proto](../proto/api.proto) file.

### API CLI

The API will execute as a CLI process. The CLI will accept configuration
information for hosting the gRPC service such as the host and port.

**NOTE:** To simplify the CLI for the exercise I will embed the certificate
and key in the binary using the `embed` package. This is *very* insecure and
should **NEVER** be done.

```bash
# Example CLI Usage
server --host=localhost --port=8080
```

### Available gRPC Commands

- `Start`: Start a new isolated process with the provided command and arguments
- `Stop`: Stop the process with the provided ID
- `Stat`: Return the process state of the process with the provided ID
- `Output`: Stream the output of the process with the provided ID

### Streaming Output

The library will propagate the output of the command to the API as an io.Reader.
The API will then use a combination of `io.TeeReader` and `bytes.Buffer`
stacking to create independent readers that can be used to stream the output to
multiple clients without creating a race. These readers will be used to respond
to clients through the Output stream in gRPC.

### Third Party Libraries

Requirements of the exercise require the following third party libraries. These
will likely be the only libraries required for the implementation. If the need
for additional libraries arises, I will confer with the team to determine if
these libraries are required.

- [google.golang.org/protobuf](https://pkg.go.dev/mod/google.golang.org/protobuf)
- [google.golang.org/grpc](https://pkg.go.dev/mod/google.golang.org/grpc)

### Transport Security

**NOTE:** Root and Private Key certificates will be embedded in this repository
for the purposes of the exercise. This is bad practice and a serious security
risk.

### Authentication

Authentication will use mTLS as defined in the requirements. The cipher suite
will follow recommendations from [SSL Labs](https://github.com/ssllabs/research/wiki/SSL-and-TLS-Deployment-Best-Practices#23-use-secure-cipher-suites).

I will likely only include those cipher suites that correspond to the keys and
certificates embedded in this repository.

### Authorization

There will be two authorized certificates for this exercise. The first will be a
certificate that is authorized for FULL-`$PATH` access. The second will be a
certificate that is authorized for a set of pre-defined commands. These commands
will include `ls`, `echo`, `cat`, `ping`.

Accessing these keyids will happen through the use of the `context.Context`
retrieved from the gRPC library and extracted using
`metadata.FromIncomingContext`.

This scheme will be enforced by an *allow-list* of commands hard-coded into the
API via a `map[string]bool`.

**NOTE:** In a real-world scenario, this would be a more complex authorization
scheme. For the purposes of this exercise, we will use a simple authorization
scheme as defined in the requirements.

## Client

The client will be a simple CLI application that will interact with the API over
gRPC. The commands will simply follow the same pattern as the API, with the
exception of the necessary url and port information.

**NOTE:** To simplify the CLI for the exercise I will embed the certificate
and key in the binary using the `embed` package. This is *very* insecure and
should **NEVER** be done.

```bash
# gRPC url config will be provided via environment variable for the exercise
# to simplify the CLI.
export GRCP_CONN=localhost:8080

# Example CLI Usage (Start)
client start command arg1 arg2 ...

# Example CLI Usage (Stop)
client stop 11982123 # example process id

# Example CLI Usage (Stat)
client stat 11982123 # example process id

# Example CLI Usage (Output)
client output 11982123 # example process id
```

**NOTE:** There will be minimal validation of the command and arguments. The
user is expected to correctly execute the command. This is not how I would write
this for production.

## Reproducible Builds and CI/CD

In order to ensure reproducible builds, this repository has a
`.pre-commit-config.yaml` specificed for the `pre-commit` framework, as well as
Github Actions to build and test the code on push. Ideally, there should never
be a failure on a push because it should get caught by the pre-commit hooks.

These pre-commit hooks also enforce code style and linting rules, and check for
embedded secrets.

**NOTE:** There will be embedded secrets added to this repo and the
detect-secrets baseline will be reset to account for those.

## Dependency Structure

To ensure proper separation of concerns (and eliminate cyclical dependencies)
the internal dependency structure will follow the approach below. Arrows
indicate dependency direction.

![Package dependency diagram](diagrams/depstructure.svg)

## Additional Unsupported Features

- No Shell / REPL Support
- Limited to Single Connections ONLY
- Hard Coded and Embedded Secrets
- Limited to single running instances of a command
- No included build tags (to limit os support)

---

## Additional Notes

![Design Drawing](diagrams/design_prep.jpg)
