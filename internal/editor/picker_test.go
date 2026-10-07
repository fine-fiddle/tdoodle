package editor

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
)

func TestPickerSamplesAndCancelsInEveryDrawingPhase(t *testing.T) {
	for _, tt := range []struct {
		tool   Tool
		phase  int
		target string
	}{
		{ToolText, 0, "Text colors"}, {ToolPen, 0, "Pen"},
		{ToolLine, 0, "Outline"}, {ToolLine, 1, "Outline"},
		{ToolRectangle, 0, "Outline"}, {ToolRectangle, 1, "Outline"}, {ToolRectangle, 2, "Fill"},
		{ToolOval, 0, "Outline"}, {ToolOval, 1, "Outline"}, {ToolOval, 2, "Outline"}, {ToolOval, 3, "Fill"},
	} {
		for _, exit := range []tcell.Key{tcell.KeyEnter, tcell.KeyEsc, tcell.KeyF7} {
			t.Run(fmt.Sprintf("%s/phase%d/exit%d", tt.tool, tt.phase, exit), func(t *testing.T) {
				e := newTestEditor(t)
				e.SwitchTool(tt.tool)
				e.Cursor = canvas.Point{X: 5, Y: 5}
				for i := 0; i < tt.phase; i++ {
					e.Enter()
					e.Move(3, 2)
				}
				source := canvas.Point{X: 1, Y: 1}
				cell := canvas.Cell{Rune: '#', FG: canvas.Orange, BG: canvas.Blue}
				e.Doc.Set(source, cell)
				cells := append([]canvas.Cell(nil), e.Doc.Cells...)
				cursor, offset, shape, preview := e.Cursor, e.Offset, e.shape(), e.Preview()
				text, outline, fill, pen := e.Text, e.Outline, e.Fill, e.Pen
				key(e, tcell.KeyF7)
				e.Move(source.X-e.PickerCursor.X, source.Y-e.PickerCursor.Y)
				for _, event := range []*tcell.EventKey{
					tcell.NewEventKey(tcell.KeyRune, "X", tcell.ModNone),
					tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone),
					tcell.NewEventKey(tcell.KeyDelete, "", tcell.ModNone),
					tcell.NewEventKey(tcell.KeyBackspace, "", tcell.ModNone),
				} {
					e.HandleKey(event, time.Now())
				}
				if !e.Picker || e.DisplayCursor() != source || e.Cursor != cursor || e.Phase != tt.phase ||
					!reflect.DeepEqual(shape, e.shape()) || !reflect.DeepEqual(preview, e.Preview()) {
					t.Fatal("picker navigation moved the drawing or changed the preview")
				}
				if status := e.Status(); !strings.Contains(status.Text, "PICK "+tt.target) || !strings.Contains(status.Text, "# orange/blue") {
					t.Fatalf("picker status lost target or source: %+v", status)
				}
				key(e, exit)
				if e.Picker || e.Tool != tt.tool || e.Cursor != cursor || e.Offset != offset || e.Phase != tt.phase || !reflect.DeepEqual(shape, e.shape()) {
					t.Fatal("closing picker did not restore the drawing operation and view")
				}
				if exit == tcell.KeyEnter {
					picked := canvas.Paint{Mode: canvas.PaintGlyph, Glyph: cell.Rune, FG: cell.FG, BG: cell.BG}
					switch tt.target {
					case "Pen":
						pen = picked
					case "Outline":
						outline = picked
					case "Fill":
						fill = picked
					}
					text.FG, text.BG = cell.FG, cell.BG
				}
				if e.Text != text || e.Outline != outline || e.Fill != fill || e.Pen != pen {
					t.Fatal("picker changed the wrong brush or changed brushes on cancellation")
				}
				if !reflect.DeepEqual(cells, e.Doc.Cells) || len(e.undo) != 0 || len(e.redo) != 0 || e.Revision != 0 {
					t.Fatal("sampling or cancellation edited cells or history")
				}
			})
		}
	}
}

func TestPickerReadsCanvasBeneathPreviewAndKeepsSpacesLiteral(t *testing.T) {
	for _, glyph := range []rune{'@', ' '} {
		t.Run(fmt.Sprintf("glyph%d", glyph), func(t *testing.T) {
			e := newTestEditor(t)
			source := canvas.Point{X: 3, Y: 2}
			cell := canvas.Cell{Rune: glyph, FG: canvas.Green, BG: canvas.Violet}
			e.Doc.Set(source, cell)
			e.SwitchTool(ToolLine)
			e.Cursor = canvas.Point{X: 1, Y: 2}
			e.Enter()
			e.Move(4, 0)
			if e.Preview()[source].Rune == cell.Rune {
				t.Fatal("test source was not covered by a different preview glyph")
			}
			key(e, tcell.KeyF7)
			e.Move(-2, 0)
			key(e, tcell.KeyEnter)
			if e.Outline.Mode != canvas.PaintGlyph || e.Outline.Glyph != glyph || e.Revision != 0 {
				t.Fatal("picker sampled the preview or interpreted a space as a brush-mode toggle")
			}
			key(e, tcell.KeyEnter)
			for x := 1; x <= 5; x++ {
				if got := e.Doc.At(canvas.Point{X: x, Y: 2}); got != cell {
					t.Fatalf("committed sampled brush at x=%d: got %+v, want %+v", x, got, cell)
				}
			}
			e.SwitchTool(ToolPen)
			if e.Pen.FG != cell.FG || e.Pen.BG != cell.BG {
				t.Fatal("tool switching reset sampled color defaults")
			}
		})
	}
}

func TestPickerCanPaintSampledFillWithoutChangingOutline(t *testing.T) {
	e := newTestEditor(t)
	e.SwitchTool(ToolRectangle)
	e.Cursor = canvas.Point{X: 2, Y: 2}
	e.Enter()
	e.Move(4, 4)
	e.Enter()
	source := canvas.Point{X: 10, Y: 10}
	cell := canvas.Cell{Rune: '.', FG: canvas.Red, BG: canvas.Blue}
	e.Doc.Set(source, cell)
	outline := e.Outline
	key(e, tcell.KeyF7)
	e.Move(source.X-e.PickerCursor.X, source.Y-e.PickerCursor.Y)
	key(e, tcell.KeyEnter)
	key(e, tcell.KeyEnter)
	if e.Doc.At(canvas.Point{X: 4, Y: 4}) != cell || e.Outline != outline || len(e.undo) != 1 {
		t.Fatal("sampled fill did not commit independently as one shape operation")
	}
}

func TestPickerEndsPenStrokeWithoutPaintingAndKeepsUndoGrouping(t *testing.T) {
	e := newTestEditor(t)
	source := canvas.Point{X: 4, Y: 3}
	cell := canvas.Cell{Rune: '@', FG: canvas.Orange, BG: canvas.Blue}
	e.Doc.Set(source, cell)
	e.SwitchTool(ToolPen)
	e.Type('#')
	e.Enter()
	e.Move(1, 0)
	cells := append([]canvas.Cell(nil), e.Doc.Cells...)
	revision := e.Revision
	key(e, tcell.KeyF7)
	if e.PenDown || len(e.undo) != 1 || len(e.stroke) != 0 {
		t.Fatal("entering picker did not lift and finish the whole pen stroke")
	}
	e.Move(3, 3)
	for _, event := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, "X", tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone),
		tcell.NewEventKey(tcell.KeyDelete, "", tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace, "", tcell.ModNone),
	} {
		e.HandleKey(event, time.Now())
	}
	key(e, tcell.KeyEnter)
	if e.PenDown || e.Pen.Glyph != '@' || e.Revision != revision || !reflect.DeepEqual(cells, e.Doc.Cells) || len(e.undo) != 1 {
		t.Fatal("sampling painted artwork or added an undo operation")
	}
	e.Enter()
	e.Move(1, 0)
	e.Enter()
	if len(e.undo) != 2 || e.Doc.At(canvas.Point{X: 1}) != cell || e.Doc.At(canvas.Point{X: 2}) != cell {
		t.Fatal("new stroke did not use the sampled brush as one undo operation")
	}
	e.Undo()
	if !reflect.DeepEqual(cells, e.Doc.Cells) {
		t.Fatal("undoing the sampled stroke did not restore the previous stroke")
	}
	e.Undo()
	if e.Doc.At(canvas.Point{}) != canvas.Blank() || e.Doc.At(canvas.Point{X: 1}) != canvas.Blank() || e.Doc.At(source) != cell {
		t.Fatal("original pen stroke lost its undo grouping")
	}
}

func TestPickerPreservesRedoHistory(t *testing.T) {
	e := newTestEditor(t)
	e.Type('A')
	e.Undo()
	revision := e.Revision
	key(e, tcell.KeyF7)
	key(e, tcell.KeyEnter)
	if len(e.redo) != 1 || len(e.undo) != 0 || e.Revision != revision {
		t.Fatal("sampling changed the redo branch")
	}
	e.Redo()
	if e.Doc.At(canvas.Point{}).Rune != 'A' {
		t.Fatal("sampling prevented redo of existing artwork")
	}
}

func TestPickerNavigationAndResizeFollowSamplerAndRestoreDrawingView(t *testing.T) {
	e := newTestEditor(t)
	e.Cursor = canvas.Point{X: 29, Y: 14}
	e.Resize(8, 5)
	key(e, tcell.KeyF7)
	key(e, tcell.KeyHome)
	if e.PickerCursor != (canvas.Point{X: 0, Y: 14}) || e.Offset.X != 0 {
		t.Fatal("Home did not follow the sampler")
	}
	key(e, tcell.KeyPgUp)
	key(e, tcell.KeyEnd)
	if e.PickerCursor != (canvas.Point{X: 29, Y: 10}) || e.Offset.X != 22 {
		t.Fatal("page movement or End moved the wrong cursor")
	}
	key(e, tcell.KeyHome)
	for i := 0; i < 4; i++ {
		key(e, tcell.KeyPgUp)
	}
	key(e, tcell.KeyLeft)
	key(e, tcell.KeyUp)
	e.Resize(6, 3)
	if e.Cursor != (canvas.Point{X: 29, Y: 14}) || e.PickerCursor != (canvas.Point{}) || e.Offset != (canvas.Point{}) {
		t.Fatal("picker navigation or resize changed the drawing cursor or exceeded canvas bounds")
	}
	key(e, tcell.KeyEsc)
	if e.Offset != (canvas.Point{X: 24, Y: 13}) || e.DisplayCursor() != e.Cursor {
		t.Fatal("closing picker did not restore a visible drawing cursor after resize")
	}
}

func TestPickerExitsForOtherModesAndPreservesGlobalActions(t *testing.T) {
	for _, tt := range []struct {
		name                           string
		key                            tcell.Key
		picker, help, palette, toolbar bool
		phase                          int
		action                         Action
	}{
		{"help", tcell.KeyF8, false, true, false, false, 1, ActionNone},
		{"colors", tcell.KeyF6, false, false, true, false, 1, ActionNone},
		{"ctrl-colors", tcell.KeyCtrlP, false, false, true, false, 1, ActionNone},
		{"toolbar", tcell.KeyTab, false, false, false, true, 1, ActionNone},
		{"tool", tcell.KeyF1, false, false, false, false, 0, ActionNone},
		{"undo", tcell.KeyCtrlZ, false, false, false, false, 0, ActionNone},
		{"redo", tcell.KeyCtrlY, false, false, false, false, 0, ActionNone},
		{"save", tcell.KeyCtrlS, true, false, false, false, 1, ActionSave},
		{"refresh", tcell.KeyCtrlL, true, false, false, false, 1, ActionRefresh},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEditor(t)
			e.SwitchTool(ToolLine)
			e.Enter()
			e.Move(4, 2)
			cursor := e.Cursor
			key(e, tcell.KeyF7)
			e.Move(5, 4)
			if action := key(e, tt.key); action != tt.action || e.Picker != tt.picker || e.Help != tt.help ||
				(e.Palette != 0) != tt.palette || e.Toolbar != tt.toolbar || e.Phase != tt.phase || e.Cursor != cursor {
				t.Fatalf("unexpected picker transition for %s", tt.name)
			}
		})
	}
}

func TestToolbarPickerPrecedesHelpAndWrapsAtLastItem(t *testing.T) {
	e := newTestEditor(t)
	key(e, tcell.KeyTab)
	key(e, tcell.KeyLeft)
	if e.ToolbarIndex != 7 {
		t.Fatal("toolbar did not wrap to Help as its last item")
	}
	key(e, tcell.KeyRight)
	if e.ToolbarIndex != 0 {
		t.Fatal("toolbar did not wrap past Help to Text")
	}
	key(e, tcell.KeyLeft)
	key(e, tcell.KeyLeft)
	if e.ToolbarIndex != 6 {
		t.Fatal("Picker did not immediately precede Help")
	}
	key(e, tcell.KeyEnter)
	if !e.Picker || e.Help || e.Toolbar {
		t.Fatal("toolbar Picker did not activate the sampler")
	}
	key(e, tcell.KeyF7)
	key(e, tcell.KeyTab)
	key(e, tcell.KeyLeft)
	key(e, tcell.KeyEnter)
	if !e.Help || e.Picker || e.Toolbar {
		t.Fatal("last toolbar item did not activate Help")
	}
	key(e, tcell.KeyF8)
	if e.Help {
		t.Fatal("F8 did not close Help")
	}
}

func TestPickerOpensFromOverlaysWithoutLosingShape(t *testing.T) {
	for _, open := range []tcell.Key{tcell.KeyF8, tcell.KeyF6, tcell.KeyTab} {
		e := newTestEditor(t)
		e.SwitchTool(ToolLine)
		e.Enter()
		e.Move(4, 2)
		preview := e.Preview()
		key(e, open)
		key(e, tcell.KeyF7)
		if !e.Picker || e.Help || e.Toolbar || e.Palette != 0 || e.Phase != 1 || !reflect.DeepEqual(preview, e.Preview()) {
			t.Fatal("opening picker from an overlay lost the drawing operation")
		}
	}
}
