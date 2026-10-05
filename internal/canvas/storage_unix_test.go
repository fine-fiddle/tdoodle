//go:build unix

package canvas

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLoadRejectsNamedPipeWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Load(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("named pipe accepted as a drawing")
		}
	case <-time.After(time.Second):
		t.Fatal("opening named pipe blocked")
	}
}
