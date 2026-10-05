package editor

import (
	"reflect"
	"testing"

	"tdoodle/internal/canvas"
	"tdoodle/internal/geometry"
)

func newGeometryEditor(t *testing.T) *Editor {
	t.Helper()
	doc, err := canvas.New(70, 45, 2)
	if err != nil {
		t.Fatal(err)
	}
	return New(doc)
}

func TestGeometryOvalCirclePhaseContinuity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		center canvas.Point
		radius canvas.Point
	}{
		{"odd horizontal radius", canvas.Point{X: 30, Y: 20}, canvas.Point{X: 5}},
		{"odd tilted radius", canvas.Point{X: 30, Y: 20}, canvas.Point{X: 7, Y: 3}},
		{"offscreen perpendicular handle", canvas.Point{X: 3, Y: 2}, canvas.Point{X: 9, Y: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newGeometryEditor(t)
			e.SwitchTool(ToolOval)
			e.Cursor = tc.center
			e.Enter()
			e.Move(tc.radius.X, tc.radius.Y)
			circle := e.Preview()
			e.Enter()
			if e.Phase != 2 {
				t.Fatalf("phase = %d, want second radius phase", e.Phase)
			}
			if !reflect.DeepEqual(circle, e.Preview()) {
				t.Fatal("confirming the first radius changed the exact circle")
			}
			want := geometry.Perpendicular(geometry.Vector{X: float64(tc.radius.X), Y: float64(tc.radius.Y)}, 2)
			if e.radius2 != want {
				t.Fatalf("virtual second radius = %v, want %v", e.radius2, want)
			}
			if e.Revision != 0 || len(e.undo) != 0 {
				t.Fatal("radius selection wrote preview cells or undo history")
			}
		})
	}
}

func TestGeometryOvalSecondMovementRetainsFractionalRadius(t *testing.T) {
	e := newGeometryEditor(t)
	e.SwitchTool(ToolOval)
	e.Cursor = canvas.Point{X: 30, Y: 20}
	e.Enter()
	e.Move(5, 0)
	e.Enter()
	e.Move(3, -1)
	want := geometry.Vector{X: 3, Y: 1.5}
	if e.radius2 != want {
		t.Fatalf("moving rounded handle produced radius %v, want %v", e.radius2, want)
	}
	wantShape := geometry.Ellipse(e.anchor, e.radius1, want, e.Doc.Aspect, e.Doc.Width, e.Doc.Height)
	if !reflect.DeepEqual(e.shape(), wantShape) {
		t.Fatal("arbitrary second handle movement did not produce its affine oval")
	}
	// Returning one phase restores the integer first handle and exact circle.
	e.Backspace()
	if e.Cursor != (canvas.Point{X: 35, Y: 20}) || e.Phase != 1 {
		t.Fatalf("backspace restored cursor %v in phase %d", e.Cursor, e.Phase)
	}
	e.Enter()
	if e.radius2 != (geometry.Vector{Y: 2.5}) {
		t.Fatalf("reconfirming first radius retained stale oval handle %v", e.radius2)
	}
}

func TestGeometryFillMovementAndBackspacePreserveOval(t *testing.T) {
	e := newGeometryEditor(t)
	e.SwitchTool(ToolOval)
	e.Cursor = canvas.Point{X: 3, Y: 3}
	e.Enter()
	e.Move(7, 3)
	e.Enter()
	// The virtual x coordinate begins outside the document; the visible cursor
	// starts at x=0. Additive movement must preserve that initial difference.
	e.Move(4, -1)
	secondCursor, secondRadius := e.Cursor, e.radius2
	oval := e.shape()
	e.Enter()
	e.Type('#')
	fillPreview := e.Preview()
	style := e.Fill
	e.Move(55, 25)
	if !reflect.DeepEqual(oval, e.shape()) || !reflect.DeepEqual(fillPreview, e.Preview()) {
		t.Fatal("moving cursor during fill changed the confirmed oval")
	}
	e.Backspace()
	if e.Phase != 2 || e.Cursor != secondCursor || e.radius2 != secondRadius {
		t.Fatalf("backspace lost second handle: phase %d, cursor %v, radius %v", e.Phase, e.Cursor, e.radius2)
	}
	if e.Fill != style || !reflect.DeepEqual(oval, e.shape()) {
		t.Fatal("backspace lost fill style or exact oval geometry")
	}
	e.Move(1, 0)
	if e.radius2.X != secondRadius.X+1 || e.radius2.Y != secondRadius.Y {
		t.Fatal("movement after returning to second radius did not update virtual handle")
	}
}

func TestGeometryRectangleFillCursorDoesNotMoveCorner(t *testing.T) {
	e := newGeometryEditor(t)
	e.SwitchTool(ToolRectangle)
	e.Cursor = canvas.Point{X: 5, Y: 6}
	e.Enter()
	e.Move(13, 7)
	e.Enter()
	e.Type('.')
	shape, preview, style := e.shape(), e.Preview(), e.Fill
	e.Move(35, 20)
	if !reflect.DeepEqual(shape, e.shape()) || !reflect.DeepEqual(preview, e.Preview()) {
		t.Fatal("moving cursor during rectangle fill moved a confirmed corner")
	}
	e.Backspace()
	if e.Cursor != (canvas.Point{X: 18, Y: 13}) || e.Phase != 1 || e.Fill != style {
		t.Fatalf("backspace lost rectangle corner or fill style: cursor %v phase %d", e.Cursor, e.Phase)
	}
}

func TestGeometryShapesCommitAsOneUndoOperation(t *testing.T) {
	for _, tool := range []Tool{ToolLine, ToolRectangle, ToolOval} {
		t.Run(tool.String(), func(t *testing.T) {
			e := newGeometryEditor(t)
			before := append([]canvas.Cell(nil), e.Doc.Cells...)
			e.SwitchTool(tool)
			e.Cursor = canvas.Point{X: 20, Y: 15}
			e.Enter()
			e.Move(9, 4)
			if tool == ToolOval {
				e.Enter()
				e.Move(2, 1)
				e.Enter()
				e.Type('.')
			} else if tool == ToolRectangle {
				e.Enter()
				e.Type('.')
			}
			if !reflect.DeepEqual(before, e.Doc.Cells) || e.Revision != 0 || len(e.undo) != 0 {
				t.Fatal("unfinished shape changed the document or history")
			}
			e.Enter()
			if e.Phase != 0 || e.Revision != 1 || len(e.undo) != 1 {
				t.Fatalf("shape commit: phase %d revision %d undo operations %d", e.Phase, e.Revision, len(e.undo))
			}
			after := append([]canvas.Cell(nil), e.Doc.Cells...)
			if reflect.DeepEqual(before, after) {
				t.Fatal("committing the shape changed no cells")
			}
			e.Undo()
			if !reflect.DeepEqual(before, e.Doc.Cells) {
				t.Fatal("one undo did not remove the whole shape")
			}
			e.Redo()
			if !reflect.DeepEqual(after, e.Doc.Cells) {
				t.Fatal("one redo did not restore the whole shape")
			}
		})
	}
}

func TestGeometryBackspaceAndCancellationDoNotWrite(t *testing.T) {
	for _, tool := range []Tool{ToolLine, ToolRectangle, ToolOval} {
		t.Run(tool.String(), func(t *testing.T) {
			e := newGeometryEditor(t)
			before := append([]canvas.Cell(nil), e.Doc.Cells...)
			e.SwitchTool(tool)
			e.Cursor = canvas.Point{X: 15, Y: 12}
			e.Enter()
			e.Move(11, 5)
			if tool == ToolOval {
				e.Enter()
				e.Move(3, 0)
				e.Enter()
			} else if tool == ToolRectangle {
				e.Enter()
			}
			for e.Phase > 0 {
				e.Backspace()
				if !reflect.DeepEqual(before, e.Doc.Cells) || len(e.undo) != 0 {
					t.Fatal("backspace wrote unfinished shape")
				}
			}
			if e.Cursor != (canvas.Point{X: 15, Y: 12}) {
				t.Fatalf("backing out all phases did not restore anchor: %v", e.Cursor)
			}
			e.Enter()
			e.Move(5, 2)
			e.cancel()
			if e.Phase != 0 || !reflect.DeepEqual(before, e.Doc.Cells) || len(e.undo) != 0 || e.Revision != 0 {
				t.Fatal("cancelling an unfinished shape changed the document")
			}
		})
	}
}
