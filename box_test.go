package sandbox

import (
	"context"
	"encoding/gob"
	"io"
	"os"
	"testing"
	"time"
)

func Test_TempDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b, err := New(ctx, time.Minute*5)
	if err != nil {
		t.Fatal(err)
	}

	defer b.Cleanup()

	info, err := os.Stat(b.tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", b.tempDir)
	}
}

func Test_Box_Start_Output(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	box, err := New(ctx, time.Minute*5)
	if err != nil {
		t.Fatal(err)
	}
	defer box.Cleanup()

	id, err := box.Start("./test/bin/reflector")
	if err != nil {
		t.Fatal(err)
	}

	output, err := box.Output(id)
	if err != nil {
		t.Fatal(err)
	}

	defer output.Close()

	d := gob.NewDecoder(output)
	for i := 0; i < 2; i++ {
		if i > 0 {
			err = box.Stop(id)
			if err != nil {
				t.Fatal(err)
			}
		}

		info := Info{}
		err = d.Decode(&info)
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}

		if i == 0 && info.Terminated {
			t.Fatal("first result should not be terminated")
		}

		if i == 1 && !info.Terminated {
			t.Fatal("second result should be terminated")
		}

		if info.PID > 100 {
			t.Fatal("improper isolation")
		}

		t.Log(info)
	}

	status, err := box.Stat(id)
	if err != nil {
		t.Fatal(err)
	}

	if !status.Exited {
		t.Fatal("expected command to be stopped")
	}
}
