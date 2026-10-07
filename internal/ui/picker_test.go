package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
	"tdoodle/internal/editor"
)

func TestPickerRevealsSourceAndKeepsDrawingHandleVisible(t *testing.T) {
	e := newEditor(t)
	source := canvas.Point{X: 3, Y: 2}
	cell := canvas.Cell{Rune: '@', FG: canvas.Red, BG: canvas.Blue}
	e.Doc.Set(source, cell)
	e.SwitchTool(editor.ToolLine)
	e.Cursor = canvas.Point{X: 1, Y: 2}
	e.Enter()
	e.Move(4, 0)
	preview := e.Preview()
	cells := append([]canvas.Cell(nil), e.Doc.Cells...)
	e.TogglePicker()
	e.Move(-2, 0)
	s := newScreen(200, 8, 256)
	Render(s, e)
	glyph, style, _ := s.buffer.Get(3, 2)
	if glyph != "@" || !style.HasReverse() || !style.HasBold() || style.GetForeground() != ColorFor(cell.FG, false, 256, false) {
		t.Fatal("sampler did not reveal and highlight the underlying source cell")
	}
	if glyph, style, _ := s.buffer.Get(5, 2); glyph != "-" || !style.HasUnderline() || style.HasReverse() {
		t.Fatal("frozen drawing handle was not kept visible independently of the sampler")
	}
	if s.cursor != source || e.Cursor != (canvas.Point{X: 5, Y: 2}) || !reflect.DeepEqual(preview, e.Preview()) || !reflect.DeepEqual(cells, e.Doc.Cells) {
		t.Fatal("rendering sampler moved or edited the drawing")
	}
	row := s.row(7)
	for _, want := range []string{"Picker [F7]", "Help [F8]", "PICK Outline: @ red/blue", "Enter: sample", "Esc: cancel"} {
		if !strings.Contains(row, want) {
			t.Fatalf("picker row lost %q: %q", want, row)
		}
	}
	if strings.Index(row, "Picker [F7]") > strings.Index(row, "Help [F8]") {
		t.Fatal("Help was not last")
	}
	for _, item := range []string{"Picker [F7]", "Help [F8]"} {
		_, style, _ := s.buffer.Get(strings.Index(row, item), 7)
		if style.HasReverse() != (item == "Picker [F7]") {
			t.Fatalf("incorrect active item styling for %s", item)
		}
	}
	normal := newScreen(80, 8, 256)
	Render(normal, e)
	for _, want := range []string{"PREVIEW", "@ red/blue", "Enter: sample", "Esc: cancel"} {
		if !strings.Contains(normal.row(7), want) {
			t.Fatalf("80-column picker lost %q: %q", want, normal.row(7))
		}
	}
	e.HandleKey(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone), time.Now())
	Render(s, e)
	if glyph, _, _ := s.buffer.Get(3, 2); glyph != "-" || s.cursor != e.Cursor {
		t.Fatal("canceling picker did not restore preview and drawing cursor")
	}
}

func TestPickerRendersOnNarrowScreensAndFollowsViewport(t *testing.T) {
	for _, colors := range []int{0, 8, 16, 256} {
		for _, width := range []int{0, 1, 2, 3, 7, 10, 24, 40, 80, 160} {
			t.Run(fmt.Sprintf("%d-colors/%d-columns", colors, width), func(t *testing.T) {
				e := newEditor(t)
				e.SwitchTool(editor.ToolRectangle)
				e.Enter()
				e.Move(4, 3)
				e.Enter()
				e.TogglePicker()
				e.Move(20, 10)
				e.Resize(width, 5)
				s := newScreen(width, 5, colors)
				Render(s, e)
				if width >= 7 && !strings.Contains(s.row(4), "PICK") {
					t.Fatalf("missing picker state: %q", s.row(4))
				}
				if width >= 24 && !strings.Contains(s.row(4), "Enter: sample") {
					t.Fatalf("missing sample action: %q", s.row(4))
				}
				if width > 0 && s.cursor != (canvas.Point{X: e.PickerCursor.X - e.Offset.X, Y: e.PickerCursor.Y - e.Offset.Y}) {
					t.Fatal("terminal cursor followed the drawing instead of the sampler")
				}
				if s.outside || s.shown != 1 || e.Cursor != (canvas.Point{X: 4, Y: 3}) || e.Phase != 2 {
					t.Fatal("picker rendering exceeded the screen or moved the shape")
				}
			})
		}
	}
}
