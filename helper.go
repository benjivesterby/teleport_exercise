package sandbox

import (
	_ "embed"
	"os"
	"path/filepath"
)

// NOTE: The naming of constants here is to keep them from being
// exported to users of the library since they are not part of the
// public API.
const (
	// helperCmd is the name of the helper binary
	helperCmd = "prochelper"

	// constraintsConfig is the name of the resource limits file
	// TODO: This should probably be a configurable option. Hard coded for now.
	constraintsConfig = "constraints.json"

	// sandboxPattern is the name of the sandbox directory
	sandboxPattern = "sandbox"
)

//go:generate go build ./internal/prochelper/

//go:embed prochelper
var helper []byte

//go:embed constraints.json
var constraints []byte

func deployHelper() (tempdir, helperpath string, err error) {
	// create a temp directory for use with this sandbox
	tempdir, err = os.MkdirTemp(os.TempDir(), sandboxPattern)
	if err != nil {
		return "", "", err
	}

	// Write the helper binary to the temp directory
	// with executable permissions.
	// nolint:gosec
	err = os.WriteFile(filepath.Join(tempdir, helperCmd), helper, 0700)
	if err != nil {
		return "", "", err
	}

	// Write the resource limits to the temp directory
	err = os.WriteFile(filepath.Join(tempdir, constraintsConfig), constraints, 0600)
	if err != nil {
		return "", "", err
	}

	helperpath = filepath.Join(tempdir, helperCmd)

	return tempdir, helperpath, nil
}
