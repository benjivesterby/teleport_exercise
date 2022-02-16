# Sandbox

Exercise based on the requirements [here](https://github.com/gravitational/careers/blob/main/challenges/systems/challenge.md)

Design document is located [here](docs/design.md)

## Styling

For styling code should follow the linter styling rules enforced by the
golangci-lint config (`.golangci.yml`) which are opinionated, but consistent.
For non-linter supported styling I use the [Go style
guide](https://github.com/golang/go/wiki/CodeReviewComments).

> NOTE: ALL Commands for the library and service should MUST be executed as a
> user with root/sudo privileges because of the requirements for isolation and
> cgroup creation.

This requirement does *NOT* apply to the `client` application.

## Library Spec

The external API for the library is listed in the design document [here](docs/design.md#exported-api).

## Testing the Library

### Unit Tests

To execute unit tests, run the following command from the root of the project:

```go
go test -race -failfast -covermode=atomic -cover -run ./...
```

### Manual Testing

To assist with testing the *library* a `commander` binary is provided for manual
testing.

To build the binary: `go build -o commander ./tools/commander`

To use the binary: `./commander <cmd> <args>`

## Cleaning the Environment

There are several built binaries that are included in the repository in a
pre-built state, along with protobuf definitions. To clean the environment, run
the following command from the root of the project:

```go
go generate ./...
```

This will re-create the protobuf definitions and the binaries that are included
in the repository.

**NOTE:** This does not include the actual artifacts like the `server` or
`client`. Those must be built using the commands which will be documented in
a future update.
