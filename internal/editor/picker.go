package editor

import "tdoodle/internal/canvas"

// DisplayCursor is the cursor followed by the viewport and terminal renderer.
// The drawing cursor stays fixed while a picker cursor moves independently.
func (e *Editor) DisplayCursor() canvas.Point {
	if e.Picker {
		return e.PickerCursor
	}
	return e.Cursor
}

func (e *Editor) TogglePicker() {
	if e.Picker {
		e.closePicker()
		return
	}
	e.FinishStroke()
	e.PenDown = false
	e.PickerCursor, e.pickerOffset = e.Cursor, e.Offset
	e.Picker = true
	e.Palette, e.Help, e.Toolbar = 0, false, false
	e.ensureVisible()
}

func (e *Editor) closePicker() {
	if !e.Picker {
		return
	}
	e.Picker = false
	e.Offset = e.pickerOffset
	e.ensureVisible()
}

func (e *Editor) samplePicker() {
	cell := e.Doc.At(e.PickerCursor)
	brush := e.brush()
	if e.Tool != ToolText {
		brush.Mode, brush.Glyph = canvas.PaintGlyph, cell.Rune
	}
	brush.FG, brush.BG = cell.FG, cell.BG
	// Palette changes use Text's colors as the defaults for later tool choices.
	e.Text.FG, e.Text.BG = cell.FG, cell.BG
	e.closePicker()
}

func (e *Editor) pickerStatus(pending bool) StatusInfo {
	cell := e.Doc.At(e.PickerCursor)
	paint := canvas.Paint{Mode: canvas.PaintGlyph, Glyph: cell.Rune}
	target := "Outline"
	switch {
	case e.Tool == ToolText:
		target = "Text colors"
	case e.Tool == ToolPen:
		target = "Pen"
	case e.brush() == &e.Fill:
		target = "Fill"
	}
	status := StatusInfo{
		Text: "PICK " + target + ": " + paintName(paint) + " " + string(cell.FG) + "/" + string(cell.BG) +
			" | Arrows: move | Enter: sample | Esc: cancel",
		Compact:  "PICK " + paintName(paint) + " " + string(cell.FG) + "/" + string(cell.BG) + " | Enter: sample | Esc: cancel",
		Minimal:  "PICK | Enter: sample",
		Fallback: "PICK",
		Warning:  pending,
	}
	if pending {
		status.Text = "PREVIEW paused | " + status.Text
		status.Compact = "PREVIEW | " + status.Compact
	}
	return status
}
