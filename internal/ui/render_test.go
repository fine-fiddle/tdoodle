package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/clipperhouse/displaywidth"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"

	"tdoodle/internal/canvas"
	"tdoodle/internal/editor"
)

type fakeScreen struct {
	w, h, colors int
	buffer       tcell.CellBuffer
	cursor       canvas.Point
	shown        int
	outside      bool
}

func newScreen(w, h, colors int) *fakeScreen {
	s := &fakeScreen{w: w, h: h, colors: colors, cursor: canvas.Point{X: -1, Y: -1}}
	s.buffer.Resize(w, h)
	return s
}

func (s *fakeScreen) Size() (int, int)    { return s.w, s.h }
func (s *fakeScreen) Colors() int         { return s.colors }
func (s *fakeScreen) Show()               { s.shown++ }
func (s *fakeScreen) HideCursor()         { s.cursor = canvas.Point{X: -1, Y: -1} }
func (s *fakeScreen) ShowCursor(x, y int) { s.cursor = canvas.Point{X: x, Y: y} }

func (s *fakeScreen) Put(x, y int, text string, style tcell.Style) (string, int) {
	g := displaywidth.StringGraphemes(text)
	width := 0
	if g.Next() {
		width = g.Width()
	}
	if x < 0 || x+width > s.w || y < 0 || y >= s.h {
		s.outside = true
	}
	return s.buffer.Put(x, y, text, style)
}

func (s *fakeScreen) row(y int) string {
	var row strings.Builder
	for x := 0; x < s.w; x++ {
		text, _, _ := s.buffer.Get(x, y)
		row.WriteString(text)
	}
	return row.String()
}

func newEditor(t *testing.T) *editor.Editor {
	t.Helper()
	doc, err := canvas.New(80, 40, 2)
	if err != nil {
		t.Fatal(err)
	}
	return editor.New(doc)
}

func TestPreviewIsOnlyAnOverlayAndCancellationRestoresCanvas(t *testing.T) {
	e := newEditor(t)
	e.Doc.Set(canvas.Point{X: 3, Y: 2}, canvas.Cell{Rune: '@', FG: canvas.Green, BG: canvas.Blue})
	before := append([]canvas.Cell(nil), e.Doc.Cells...)
	e.SwitchTool(editor.ToolLine)
	e.Cursor = canvas.Point{X: 1, Y: 2}
	e.Enter()
	e.Move(4, 0)
	s := newScreen(12, 6, 256)
	Render(s, e)
	text, _, _ := s.buffer.Get(3, 2)
	if text != "-" {
		t.Fatalf("preview at existing cell = %q, want line", text)
	}
	if !reflect.DeepEqual(before, e.Doc.Cells) {
		t.Fatal("render changed committed cells")
	}
	e.HandleKey(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone), time.Now())
	Render(s, e)
	text, _, _ = s.buffer.Get(3, 2)
	if text != "@" || e.Phase != 0 {
		t.Fatalf("cancelled preview left %q, phase %d", text, e.Phase)
	}
	if !reflect.DeepEqual(before, e.Doc.Cells) {
		t.Fatal("cancelled preview changed committed cells")
	}
}

func TestViewportCursorKeepsUnderlyingCharacter(t *testing.T) {
	e := newEditor(t)
	e.Cursor = canvas.Point{X: 6, Y: 4}
	e.Offset = canvas.Point{X: 5, Y: 3}
	e.Doc.Set(e.Cursor, canvas.Cell{Rune: '@', FG: canvas.Yellow, BG: canvas.Blue})
	s := newScreen(5, 4, 256)
	Render(s, e)
	text, style, _ := s.buffer.Get(1, 1)
	if text != "@" || !style.HasReverse() {
		t.Fatalf("cursor cell = %q reverse=%v, want preserved @ with contrast", text, style.HasReverse())
	}
	if s.cursor != (canvas.Point{X: 1, Y: 1}) || e.Doc.At(e.Cursor).Rune != '@' {
		t.Fatalf("cursor position or document was changed: %+v", s.cursor)
	}
}

func TestStatusUsesOneRowAndPrioritizesMessage(t *testing.T) {
	e := newEditor(t)
	e.Message = "Save failed: permission denied"
	s := newScreen(40, 4, 16)
	Render(s, e)
	status := s.row(3)
	if !strings.Contains(status, e.Message) || !strings.HasPrefix(status, "Text F1") {
		t.Fatalf("status did not retain active tool and message: %q", status)
	}
	if strings.TrimSpace(s.row(2)) != "" {
		t.Fatalf("status occupied a canvas row: %q", s.row(2))
	}
	if s.shown != 1 || s.outside {
		t.Fatalf("shows=%d out of bounds=%v", s.shown, s.outside)
	}
}

func TestRenderingSmallScreensNeverWritesOutside(t *testing.T) {
	e := newEditor(t)
	e.Message = "Save failed: drawing-界.tdoodle"
	for _, size := range [][2]int{{0, 0}, {0, 4}, {1, 1}, {1, 2}, {2, 2}, {4, 3}, {8, 3}, {25, 2}, {40, 4}} {
		for _, mode := range []string{"drawing", "palette", "help"} {
			t.Run(fmt.Sprintf("%s/%dx%d", mode, size[0], size[1]), func(t *testing.T) {
				e.Palette = 0
				e.Help = false
				if mode == "palette" {
					e.Palette = 1
				}
				if mode == "help" {
					e.Help = true
				}
				s := newScreen(size[0], size[1], 16)
				Render(s, e)
				if s.outside || s.shown != 1 {
					t.Fatalf("%dx%d: out of bounds=%v, shows=%d", size[0], size[1], s.outside, s.shown)
				}
			})
		}
	}
}

func TestToolbarSelectionAndActiveToolAreVisible(t *testing.T) {
	e := newEditor(t)
	e.SwitchTool(editor.ToolPen)
	e.Toolbar, e.ToolbarIndex = true, 6
	s := newScreen(140, 4, 16)
	Render(s, e)
	row := s.row(3)
	for _, item := range []string{"Pen [F5]", "Help [F7]"} {
		x := strings.Index(row, item)
		if x < 0 {
			t.Fatalf("missing %q in %q", item, row)
		}
		_, style, _ := s.buffer.Get(x, 3)
		if !style.HasReverse() {
			t.Fatalf("%q lacks a visible selection", item)
		}
		if item == "Help [F7]" && !style.HasUnderline() {
			t.Fatal("toolbar arrow selection is not underlined")
		}
	}
	if s.cursor.X != -1 {
		t.Fatal("drawing cursor should be hidden in toolbar")
	}
}

func TestPaletteShowsCurrentFillAndArrowSelection(t *testing.T) {
	e := newEditor(t)
	e.SwitchTool(editor.ToolRectangle)
	e.Phase = 2
	e.Fill.FG, e.Outline.FG = canvas.Blue, canvas.White
	e.Palette, e.PaletteIndex = 1, 0
	s := newScreen(160, 5, 256)
	Render(s, e)
	row := s.row(4)
	blue := strings.Index(row, "5 Blue")
	red := strings.Index(row, "[1 Red]")
	white := strings.Index(row, "9 White")
	if blue < 0 || red < 0 || white < 0 {
		t.Fatalf("missing palette labels in %q", row)
	}
	_, blueStyle, _ := s.buffer.Get(blue, 4)
	_, redStyle, _ := s.buffer.Get(red, 4)
	_, whiteStyle, _ := s.buffer.Get(white, 4)
	if !blueStyle.HasBold() || whiteStyle.HasBold() || !redStyle.HasUnderline() {
		t.Fatal("palette must identify current fill color and independent arrow selection")
	}
	if s.cursor.X != -1 || e.Phase != 2 {
		t.Fatal("palette should hide cursor and preserve operation")
	}
}

func TestLinuxForegroundSwatchesUseForegroundPalette(t *testing.T) {
	t.Setenv("TERM", "linux")
	e := newEditor(t)
	e.Palette = 1
	s := newScreen(160, 4, 16)
	Render(s, e)
	row := s.row(3)
	x := strings.Index(row, "1 Red")
	_, style, _ := s.buffer.Get(x, 3)
	if style.GetForeground() != color.Red {
		t.Fatalf("foreground swatch = %v, want 16-color red", style.GetForeground())
	}
	for x := 0; x < s.w; x++ {
		_, style, _ := s.buffer.Get(x, 3)
		bg := style.GetBackground()
		if bg != color.Default && (bg < color.XTerm0 || bg > color.XTerm7) {
			t.Fatalf("Linux foreground palette swatch uses unsupported background %v", bg)
		}
	}
	if !strings.Contains(row, "approx") {
		t.Fatal("limited palette must disclose approximations")
	}
}

func TestHelpScrollsToLicenseAndPreservesOperation(t *testing.T) {
	e := newEditor(t)
	e.SwitchTool(editor.ToolLine)
	e.Enter()
	e.Move(4, 3)
	before := append([]canvas.Cell(nil), e.Doc.Cells...)
	e.Help, e.HelpScroll = true, HelpRows(80)
	s := newScreen(80, 9, 16)
	Render(s, e)
	var contents strings.Builder
	for y := 0; y < s.h-1; y++ {
		contents.WriteString(s.row(y))
	}
	if !strings.Contains(contents.String(), "GNU GPL version 3") || !strings.Contains(contents.String(), "no warranty") {
		t.Fatalf("scrolled help is missing licensing information: %q", contents.String())
	}
	if e.Phase != 1 || !reflect.DeepEqual(before, e.Doc.Cells) || s.cursor.X != -1 {
		t.Fatal("help must preserve unfinished operation and committed cells")
	}
	e.Help = false
	Render(s, e)
	text, _, _ := s.buffer.Get(0, 0)
	if text == "t" {
		t.Fatal("closing help did not redraw artwork")
	}
}
