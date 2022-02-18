package sandbox

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func Test_deployHelper(t *testing.T) {
	tempdir, helperpath, err := deployHelper()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempdir)
	})

	info, err := os.Stat(tempdir)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", tempdir)
	}

	// Verify helper deployment
	data, err := os.ReadFile(helperpath)
	if err != nil {
		t.Fatal(err)
	}

	deployed := sha256.Sum256(data)
	embedded := sha256.Sum256(helper)
	if !bytes.Equal(deployed[:], embedded[:]) {
		t.Fatalf("expected sha256 to be the same")
	}

	// verify constraint deployment
	data, err = os.ReadFile(filepath.Join(tempdir, constraintsConfig))
	if err != nil {
		t.Fatal(err)
	}

	deployed = sha256.Sum256(data)
	embedded = sha256.Sum256(constraints)
	if !bytes.Equal(deployed[:], embedded[:]) {
		t.Fatalf("expected sha256 to be the same")
	}
}
