package sandbox

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

//go:generate go build -o tools/reflector/ tools/reflector/reflector.go

func init() {
	gob.Register(Info{})
}

type Info struct {
	PID        int
	UID        int
	GID        int
	Terminated bool
}

func (i Info) String() string {
	stat := "RUNNING"
	if i.Terminated {
		stat = "STOPPED"
	}

	return fmt.Sprintf(
		"Process %d: Status %s | UID: %d | GID: %d",
		i.PID,
		stat,
		i.UID,
		i.GID,
	)
}

func Test_Reflect(t *testing.T) {
	tempdir, helper, err := deployHelper()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempdir)
	})

	cmd, err := createCmd(
		tempdir,
		helper,
		time.Minute*5,
		"./test/bin/reflector",
	)
	if err != nil {
		t.Fatal(err)
	}

	output, ok := <-cmd.output
	if !ok {
		t.Fatal("channel close prematurely")
	}
	defer output.Close()

	d := gob.NewDecoder(output)
	for i := 0; i < 2; i++ {
		if i > 0 {
			cmd.stop <- struct{}{}
		}

		info := Info{}
		err := d.Decode(&info)
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
}

func Test_createCmd_parallel_read(t *testing.T) {
	tempdir, helper, err := deployHelper()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempdir)
	})

	info, err := createCmd(
		tempdir,
		helper,
		time.Minute*5,
		"tree",
	)
	if err != nil {
		t.Fatal(err)
	}

	allout := make(chan []byte, 5)
	defer close(allout)

	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			r, ok := <-info.output
			if !ok {
				t.Error("expected output")
			}
			defer r.Close()

			fullOutput := make([]byte, 0, 1024)
			b := make([]byte, 1024)
			for {
				n, err := r.Read(b)
				if err == io.EOF {
					break
				}

				if err != nil {
					t.Error(err)
				}
				fullOutput = append(fullOutput, b[:n]...)
			}

			// Push the output to the channel
			allout <- fullOutput
		}()
	}

	wg.Wait()

	var last []byte
	for i := 0; i < 5; i++ {
		if i == 0 {
			last = <-allout
			continue
		}

		next := <-allout
		if !bytes.Equal(last, next) {
			t.Error("outputs should be equal")
		}

		last = next
	}

	t.Logf("command output: %s", last)
}
