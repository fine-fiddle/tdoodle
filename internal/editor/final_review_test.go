package editor

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
)

func TestQuitGuardExpiryPreservesAsyncSaveError(t *testing.T) {
	for _, message := range []string{"RECOVERY FAILED: permission denied", "SAVE FAILED: permission denied"} {
		t.Run(message, func(t *testing.T) {
			e := newTestEditor(t)
			now := time.Now()
			e.HandleKey(tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl), now)
			// Recovery and signal-triggered saves can replace the guard prompt
			// without another key press in the application's event loop.
			e.Message = message
			e.Tick(now.Add(3 * time.Second))
			if e.Message != message {
				t.Fatalf("expiring quit guard cleared asynchronous error: got %q, want %q", e.Message, message)
			}
		})
	}
}
