package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
)

// Embedding supplies unused Screen methods; every method exercised by the
// application is implemented here, so tests need no terminal or private mocks.
type appScreen struct {
	tcell.Screen
	w, h     int
	events   chan tcell.Event
	rows     [][]string
	finished bool
	initErr  error
	onShow   func(*appScreen)
}

func newAppScreen(w, h int) *appScreen {
	s := &appScreen{w: w, h: h, events: make(chan tcell.Event, 128), rows: make([][]string, h)}
	for y := range s.rows {
		s.rows[y] = make([]string, w)
	}
	return s
}
func (s *appScreen) Init() error              { return s.initErr }
func (s *appScreen) Fini()                    { s.finished = true }
func (s *appScreen) Size() (int, int)         { return s.w, s.h }
func (s *appScreen) Colors() int              { return 256 }
func (s *appScreen) EventQ() chan tcell.Event { return s.events }
func (s *appScreen) DisableMouse()            {}
func (s *appScreen) EnablePaste()             {}
func (s *appScreen) Sync()                    {}
func (s *appScreen) ShowCursor(int, int)      {}
func (s *appScreen) HideCursor()              {}
func (s *appScreen) Show() {
	if s.onShow != nil {
		s.onShow(s)
	}
}
func (s *appScreen) Put(x, y int, text string, style tcell.Style) (string, int) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h || len(text) == 0 {
		return text, 0
	}
	_, n := utf8.DecodeRuneInString(text)
	s.rows[y][x] = text[:n]
	return text[n:], 1
}
func (s *appScreen) key(k tcell.Key) { s.events <- tcell.NewEventKey(k, "", tcell.ModNone) }
func (s *appScreen) text(str string) {
	s.events <- tcell.NewEventKey(tcell.KeyRune, str, tcell.ModNone)
}
func (s *appScreen) quit() { s.key(tcell.KeyCtrlC); s.key(tcell.KeyCtrlC) }

func TestApplicationSavesAndRestoresTerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.tdoodle")
	s := newAppScreen(100, 24)
	s.text("A")
	s.text("B")
	s.quit()
	tcell.ShimScreen(s)
	var out, errOut bytes.Buffer
	if err := run([]string{path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	d, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.Width != 100 || d.Height != 23 || d.At(canvas.Point{}).Rune != 'A' || d.At(canvas.Point{X: 1}).Rune != 'B' {
		t.Fatal("saved drawing differs from session")
	}
	if !s.finished || !strings.Contains(out.String(), "Saved "+path) {
		t.Fatal("terminal not restored or save not reported")
	}
}

func TestTerminalInitFailureDoesNotFinalizeUninitializedScreen(t *testing.T) {
	s := newAppScreen(100, 24)
	s.initErr = errors.New("terminal unavailable")
	tcell.ShimScreen(s)
	var out bytes.Buffer
	err := run([]string{filepath.Join(t.TempDir(), "drawing.tdoodle")}, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "terminal unavailable") || s.finished {
		t.Fatal("initialization failure was not returned safely", err)
	}
}

func TestFailedQuitSaveKeepsSessionAndArtwork(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(parent, "drawing.tdoodle")
	s := newAppScreen(120, 24)
	s.text("A")
	s.quit()
	sawFailure := false
	s.onShow = func(s *appScreen) {
		if strings.Contains(strings.Join(s.rows[s.h-1], ""), "SAVE FAILED") && !sawFailure {
			sawFailure = true
			if s.finished {
				t.Fatal("failed save closed terminal")
			}
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			s.text("B")
			s.quit()
		}
	}
	tcell.ShimScreen(s)
	var out bytes.Buffer
	if err := run([]string{"-autosave", "0", path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	d, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !sawFailure || d.At(canvas.Point{}).Rune != 'A' || d.At(canvas.Point{X: 1}).Rune != 'B' {
		t.Fatal("failed save lost drawing or did not continue")
	}
}

func TestQuitDoesNotCommitUnfinishedShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.tdoodle")
	s := newAppScreen(100, 24)
	s.text("X")
	s.key(tcell.KeyF4)
	s.key(tcell.KeyEnter)
	s.key(tcell.KeyRight)
	s.key(tcell.KeyRight)
	s.quit()
	tcell.ShimScreen(s)
	var out bytes.Buffer
	if err := run([]string{path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	d, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.At(canvas.Point{}).Rune != 'X' || d.At(canvas.Point{X: 1}) != canvas.Blank() {
		t.Fatal("quit committed unfinished preview")
	}
}

func TestPastedControlsCannotQuitOrCommitShapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.tdoodle")
	s := newAppScreen(100, 24)
	s.events <- tcell.NewEventPaste(true)
	s.text("AB")
	s.quit()
	s.key(tcell.KeyEnter)
	s.events <- tcell.NewEventPaste(false)
	s.text("C")
	s.quit()
	tcell.ShimScreen(s)
	var out bytes.Buffer
	if err := run([]string{path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	d, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for x, want := range "ABC" {
		if d.At(canvas.Point{X: x}).Rune != want {
			t.Fatal("paste control escaped guard")
		}
	}
}

func TestPickerPasteCannotPaintSampleOrDispatchControls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.tdoodle")
	doc, err := canvas.New(12, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	source := canvas.Point{X: 4, Y: 1}
	cell := canvas.Cell{Rune: '#', FG: canvas.Orange, BG: canvas.Blue}
	doc.Set(source, cell)
	if err := canvas.Save(path, doc); err != nil {
		t.Fatal(err)
	}
	s := newAppScreen(100, 24)
	s.key(tcell.KeyF7)
	for i := 0; i < 4; i++ {
		s.key(tcell.KeyRight)
	}
	s.key(tcell.KeyDown)
	s.events <- tcell.NewEventPaste(true)
	s.text("X")
	s.text("AB")
	s.key(tcell.KeyEnter)
	s.key(tcell.KeyEsc)
	s.key(tcell.KeyF1)
	s.key(tcell.KeyF8)
	s.key(tcell.KeyDelete)
	s.quit()
	s.events <- tcell.NewEventPaste(false)
	s.key(tcell.KeyEnter)
	s.text("Z")
	s.quit()
	tcell.ShimScreen(s)
	var out bytes.Buffer
	if err := run([]string{"-autosave", "0", path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	got, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Set(canvas.Point{}, canvas.Cell{Rune: 'Z', FG: cell.FG, BG: cell.BG})
	for i, want := range doc.Cells {
		if got.Cells[i] != want {
			t.Fatalf("picker/paste changed cell %d: got %+v, want %+v", i, got.Cells[i], want)
		}
	}
}

func TestApplicationPenUsesPickedBrushAtOriginalCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing.tdoodle")
	doc, err := canvas.New(12, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	cell := canvas.Cell{Rune: '@', FG: canvas.Red, BG: canvas.Blue}
	doc.Set(canvas.Point{X: 4, Y: 1}, cell)
	if err := canvas.Save(path, doc); err != nil {
		t.Fatal(err)
	}
	s := newAppScreen(100, 24)
	s.key(tcell.KeyF5)
	s.key(tcell.KeyF7)
	for i := 0; i < 4; i++ {
		s.key(tcell.KeyRight)
	}
	s.key(tcell.KeyDown)
	s.key(tcell.KeyEnter)
	s.key(tcell.KeyEnter)
	s.key(tcell.KeyRight)
	s.key(tcell.KeyEsc)
	s.quit()
	tcell.ShimScreen(s)
	var out bytes.Buffer
	if err := run([]string{"-autosave", "0", path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	got, err := canvas.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Set(canvas.Point{}, cell)
	doc.Set(canvas.Point{X: 1}, cell)
	for i, want := range doc.Cells {
		if got.Cells[i] != want {
			t.Fatalf("sampled pen brush changed cell %d: got %+v, want %+v", i, got.Cells[i], want)
		}
	}
}
