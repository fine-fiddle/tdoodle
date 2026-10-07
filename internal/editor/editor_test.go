package editor

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
)

func TestShapeStatusFollowsDrawingPhases(t *testing.T) {
	tests := []struct {
		tool  Tool
		steps []struct{ step, next, brush string }
	}{
		{ToolLine, []struct{ step, next, brush string }{
			{"Start", "start", "Outline"},
			{"End", "draw", "Outline"},
		}},
		{ToolRectangle, []struct{ step, next, brush string }{
			{"First corner", "start", "Outline"},
			{"Opposite corner", "fill", "Outline"},
			{"Fill", "draw", "Fill"},
		}},
		{ToolOval, []struct{ step, next, brush string }{
			{"Center", "start", "Outline"},
			{"Circle size", "stretch", "Outline"},
			{"Stretch", "fill", "Outline"},
			{"Fill", "draw", "Fill"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.tool.String(), func(t *testing.T) {
			e := newTestEditor(t)
			e.SwitchTool(tt.tool)
			e.Cursor = canvas.Point{X: 5, Y: 5}
			for phase, step := range tt.steps {
				status := e.Status()
				for _, want := range []string{step.step, "Enter: " + step.next, step.brush + ":"} {
					if !strings.Contains(status.Text, want) {
						t.Fatalf("phase %d: status %q is missing %q", phase, status.Text, want)
					}
				}
				if status.Warning != (phase > 0) || strings.Contains(status.Text, "PREVIEW") != (phase > 0) {
					t.Fatalf("phase %d: unexpected preview warning %+v", phase, status)
				}
				if phase > 0 {
					e.Move(2, 1)
				}
				e.Enter()
			}
			if status := e.Status(); status.Warning || strings.Contains(status.Text, "PREVIEW") || e.Phase != 0 {
				t.Fatalf("completed shape retained preview warning: %+v, phase %d", status, e.Phase)
			}
		})
	}
}

func TestTextAndPenStatusDoNotWarnAboutPreviews(t *testing.T) {
	e := newTestEditor(t)
	if status := e.Status(); status.Warning || status.Text != "white/black" {
		t.Fatalf("text status = %+v", status)
	}
	e.SwitchTool(ToolPen)
	for _, want := range []string{"Pen UP: * white/black", "Pen DOWN: * white/black", "Pen UP: * white/black"} {
		if status := e.Status(); status.Warning || status.Text != want {
			t.Fatalf("pen status = %+v, want %q without preview warning", status, want)
		}
		e.Enter()
	}
}

func newTestEditor(t *testing.T) *Editor {
	t.Helper()
	d, err := canvas.New(30, 15, 2)
	if err != nil {
		t.Fatal(err)
	}
	return New(d)
}

func key(e *Editor, k tcell.Key) Action {
	return e.HandleKey(tcell.NewEventKey(k, "", tcell.ModNone), time.Now())
}

func TestTextOverwritesWithoutShiftingOrWrapping(t *testing.T) {
	e := newTestEditor(t)
	e.Doc.Set(canvas.Point{X: 1}, canvas.Cell{Rune: '#', FG: canvas.Red, BG: canvas.Blue})
	e.Type('A')
	if e.Doc.At(canvas.Point{}).Rune != 'A' || e.Doc.At(canvas.Point{X: 1}).Rune != '#' {
		t.Fatal("text shifted neighboring artwork")
	}
	e.Cursor.X = 29
	e.Type('B')
	e.Type('C')
	if e.Cursor.X != 29 || e.Cursor.Y != 0 || e.Doc.At(e.Cursor).Rune != 'C' {
		t.Fatal("text wrapped")
	}
	e.Backspace()
	if e.Cursor.X != 28 || e.Doc.At(e.Cursor) != canvas.Blank() {
		t.Fatal("backspace did not move left and clear")
	}
	e.Enter()
	if e.Cursor != (canvas.Point{X: 0, Y: 1}) {
		t.Fatal("Enter should begin next row")
	}
	e.Type('é')
	if e.Doc.At(e.Cursor) != canvas.Blank() {
		t.Fatal("non-ASCII character accepted")
	}
}

func TestRectanglePreviewCancelAndSingleUndo(t *testing.T) {
	e := newTestEditor(t)
	e.Doc.Set(canvas.Point{X: 4, Y: 4}, canvas.Cell{Rune: 'A', FG: canvas.Red, BG: canvas.Black})
	before := append([]canvas.Cell(nil), e.Doc.Cells...)
	e.SwitchTool(ToolRectangle)
	e.Cursor = canvas.Point{X: 2, Y: 2}
	e.Enter()
	e.Move(5, 4)
	e.Type('#')
	e.Enter()
	e.Type('.')
	preview := e.Preview()
	if preview[canvas.Point{X: 4, Y: 4}].Rune != '.' {
		t.Fatal("fill missing from preview")
	}
	if !reflect.DeepEqual(before, e.Doc.Cells) {
		t.Fatal("preview changed document")
	}
	key(e, tcell.KeyEsc)
	if !reflect.DeepEqual(before, e.Doc.Cells) || len(e.undo) != 0 {
		t.Fatal("cancel changed canvas/history")
	}
	e.Cursor = canvas.Point{X: 2, Y: 2}
	e.Enter()
	e.Move(5, 4)
	e.Enter()
	e.Type('.')
	e.Enter()
	if e.Doc.At(canvas.Point{X: 4, Y: 4}).Rune != '.' || len(e.undo) != 1 {
		t.Fatal("rectangle wasn't one operation")
	}
	e.Undo()
	if !reflect.DeepEqual(before, e.Doc.Cells) {
		t.Fatal("undo did not restore entire rectangle")
	}
	e.Redo()
	if e.Doc.At(canvas.Point{X: 4, Y: 4}).Rune != '.' {
		t.Fatal("redo failed")
	}
}

func TestBrushModesHaveDistinctEffects(t *testing.T) {
	e := newTestEditor(t)
	inside := canvas.Point{X: 3, Y: 3}
	edge := canvas.Point{X: 2, Y: 2}
	original := canvas.Cell{Rune: 'X', FG: canvas.Red, BG: canvas.Blue}
	e.Doc.Set(inside, original)
	e.Doc.Set(edge, original)
	e.SwitchTool(ToolRectangle)
	e.Cursor = edge
	e.Enter()
	e.Move(3, 3)
	e.Enter()
	if _, ok := e.Preview()[inside]; ok {
		t.Fatal("transparent fill writes cells")
	}
	e.Type(' ')
	e.Fill.BG = canvas.Green
	if got := e.Preview()[inside]; got.Rune != ' ' || got.BG != canvas.Green {
		t.Fatal("literal space must paint selected background")
	}
	e.Type(' ')
	if got := e.Preview()[inside]; got != canvas.Blank() {
		t.Fatal("delete must restore canonical blank")
	}
	e.Type(' ')
	if _, ok := e.Preview()[inside]; ok {
		t.Fatal("fill cycle did not return to transparent")
	}
	e.Outline.Mode = canvas.PaintTransparent
	e.Fill.Mode = canvas.PaintGlyph
	e.Fill.Glyph = '.'
	e.Enter()
	if e.Doc.At(edge) != original {
		t.Fatal("transparent outline destroyed old boundary cell")
	}
}

func TestPaletteDoesNotChangeOutlineWhileChoosingFill(t *testing.T) {
	e := newTestEditor(t)
	e.SwitchTool(ToolRectangle)
	e.Enter()
	e.Move(4, 4)
	key(e, tcell.KeyF6)
	e.HandleKey(tcell.NewEventKey(tcell.KeyRune, "1", tcell.ModNone), time.Now())
	if e.Palette != 0 || e.Outline.FG != canvas.Red {
		t.Fatal("foreground numeric choice failed")
	}
	e.Enter()
	key(e, tcell.KeyF6)
	e.HandleKey(tcell.NewEventKey(tcell.KeyRune, "3", tcell.ModNone), time.Now())
	if e.Fill.FG != canvas.Yellow || e.Outline.FG != canvas.Red {
		t.Fatal("fill color changed outline")
	}
	key(e, tcell.KeyF6)
	key(e, tcell.KeyF6)
	e.HandleKey(tcell.NewEventKey(tcell.KeyRune, "5", tcell.ModNone), time.Now())
	if e.Fill.BG != canvas.Blue || e.Palette != 0 {
		t.Fatal("background choice failed")
	}
}

func TestPenUndoGroupsWholeStrokeAndSwitchFinishesIt(t *testing.T) {
	e := newTestEditor(t)
	e.SwitchTool(ToolPen)
	e.Type('#')
	e.Enter()
	e.Move(1, 0)
	e.Move(1, 0)
	e.Move(-1, 0)
	e.SwitchTool(ToolText)
	if e.PenDown || len(e.undo) != 1 {
		t.Fatal("switch did not finish one pen stroke")
	}
	e.Undo()
	for x := 0; x < 3; x++ {
		if e.Doc.At(canvas.Point{X: x}) != canvas.Blank() {
			t.Fatal("undo left a pen cell")
		}
	}
	e.Redo()
	for x := 0; x < 3; x++ {
		if e.Doc.At(canvas.Point{X: x}).Rune != '#' {
			t.Fatal("redo lost pen cell")
		}
	}
}

func TestUndoDuringPenAndRedoBranch(t *testing.T) {
	e := newTestEditor(t)
	e.SwitchTool(ToolPen)
	e.Enter()
	e.Move(1, 0)
	e.Undo()
	if e.PenDown || e.Doc.At(canvas.Point{}) != canvas.Blank() {
		t.Fatal("undo during active stroke failed")
	}
	e.Redo()
	e.Undo()
	e.SwitchTool(ToolText)
	e.Type('X')
	e.Redo()
	if e.Doc.At(canvas.Point{}).Rune != ' ' || e.Doc.At(canvas.Point{X: 1}).Rune != 'X' {
		t.Fatal("editing after undo should discard redo")
	}
}

func TestHelpAndPalettePreservePreview(t *testing.T) {
	e := newTestEditor(t)
	e.SwitchTool(ToolLine)
	e.Enter()
	e.Move(4, 4)
	p := e.Preview()
	key(e, tcell.KeyF8)
	key(e, tcell.KeyDown)
	key(e, tcell.KeyEsc)
	key(e, tcell.KeyF6)
	key(e, tcell.KeyEsc)
	if e.Phase != 1 || !reflect.DeepEqual(p, e.Preview()) {
		t.Fatal("overlay disturbed shape")
	}
	key(e, tcell.KeyF1)
	if e.Phase != 0 || e.Tool != ToolText || e.Preview() != nil {
		t.Fatal("tool switch didn't cancel")
	}
}

func TestResizeScrollsWithoutCropping(t *testing.T) {
	e := newTestEditor(t)
	e.Cursor = canvas.Point{X: 29, Y: 14}
	e.Doc.Set(e.Cursor, canvas.Cell{Rune: 'Z', FG: canvas.White, BG: canvas.Black})
	e.Resize(8, 5)
	if e.Doc.Width != 30 || e.Doc.Height != 15 || e.Doc.At(e.Cursor).Rune != 'Z' || e.Offset != (canvas.Point{X: 22, Y: 11}) {
		t.Fatal("resize cropped or didn't scroll")
	}
	e.Resize(40, 20)
	if e.Offset != (canvas.Point{}) || e.Doc.At(e.Cursor).Rune != 'Z' {
		t.Fatal("enlarging viewport lost data")
	}
}

func TestQuitGuardLegacyAndModernKeys(t *testing.T) {
	for _, modern := range []bool{false, true} {
		e := newTestEditor(t)
		now := time.Now()
		k := tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl)
		if modern {
			k = tcell.NewEventKeyEx(tcell.KeyRune, "c", tcell.ModCtrl, true, tcell.KeyC, 1)
		}
		if e.HandleKey(k, now) != ActionNone || e.HandleKey(k, now.Add(time.Second)) != ActionQuit {
			t.Fatal("double quit guard failed", modern)
		}
		e.HandleKey(k, now)
		key(e, tcell.KeyRight)
		if e.HandleKey(k, now.Add(time.Second)) != ActionNone {
			t.Fatal("other key did not disarm quit")
		}
		if !e.Tick(now.Add(4*time.Second)) || e.HandleKey(k, now.Add(4*time.Second)) != ActionNone {
			t.Fatal("timeout did not disarm quit")
		}
	}
}

func TestReleaseAndControlKeysCannotPaint(t *testing.T) {
	e := newTestEditor(t)
	e.HandleKey(tcell.NewEventKeyEx(tcell.KeyRune, "X", tcell.ModNone, false, 0, 1), time.Now())
	e.HandleKey(tcell.NewEventKeyEx(tcell.KeyRune, "z", tcell.ModCtrl, true, tcell.KeyZ, 1), time.Now())
	if e.Doc.At(canvas.Point{}) != canvas.Blank() {
		t.Fatal("release/control event painted a glyph")
	}
}

func TestToolbarFallback(t *testing.T) {
	e := newTestEditor(t)
	key(e, tcell.KeyTab)
	key(e, tcell.KeyRight)
	key(e, tcell.KeyEnter)
	if e.Tool != ToolOval || e.Toolbar {
		t.Fatal("toolbar fallback failed")
	}
}

func TestRevisitingFillPreservesChosenStyle(t *testing.T) {
	for _, tool := range []Tool{ToolRectangle, ToolOval} {
		e := newTestEditor(t)
		e.SwitchTool(tool)
		e.Enter()
		e.Move(4, 2)
		e.Enter()
		if tool == ToolOval {
			e.Enter()
		}
		e.TogglePalette()
		e.SetColor(canvas.Red)
		e.Type('.')
		e.Backspace()
		e.TogglePalette()
		e.SetColor(canvas.Blue)
		e.Enter()
		if e.Fill.FG != canvas.Red || e.Fill.Glyph != '.' {
			t.Fatal("revisiting fill changed chosen style", tool)
		}
	}
}
