// Package editor implements tDoodle's tools without owning the terminal or files.
package editor

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
	"tdoodle/internal/geometry"
)

type Tool int

const (
	ToolText Tool = iota
	ToolOval
	ToolRectangle
	ToolLine
	ToolPen
)

func (t Tool) String() string {
	return [...]string{"Text", "Oval", "Rectangle", "Line", "Pen"}[t]
}

type Action int

const (
	ActionNone Action = iota
	ActionSave
	ActionQuit
	ActionRefresh
)

type change struct {
	Point         canvas.Point
	Before, After canvas.Cell
}

type transaction []change

type Editor struct {
	Doc                      *canvas.Document
	Cursor, Offset           canvas.Point
	Tool                     Tool
	Phase                    int
	Text, Outline, Fill, Pen canvas.Paint
	PenDown                  bool
	Palette, PaletteIndex    int
	Toolbar                  bool
	ToolbarIndex             int
	Help                     bool
	HelpScroll               int
	Message                  string
	Revision                 uint64
	viewWidth, viewHeight    int
	anchor, corner           canvas.Point
	radius1, radius2         geometry.Vector
	secondStart              canvas.Point
	fillStarted              bool
	undo, redo               []transaction
	stroke                   map[canvas.Point]change
	quitKey                  string
	quitUntil                time.Time
}

func New(doc *canvas.Document) *Editor {
	return &Editor{
		Doc:       doc,
		Text:      canvas.Paint{Mode: canvas.PaintGlyph, FG: canvas.White, BG: canvas.Black},
		Outline:   canvas.Paint{Mode: canvas.PaintAuto, FG: canvas.White, BG: canvas.Black},
		Fill:      canvas.Paint{Mode: canvas.PaintTransparent, FG: canvas.White, BG: canvas.Black},
		Pen:       canvas.Paint{Mode: canvas.PaintGlyph, Glyph: '*', FG: canvas.White, BG: canvas.Black},
		viewWidth: doc.Width, viewHeight: doc.Height,
	}
}

// Resize changes only the viewport. The document retains every off-screen cell.
func (e *Editor) Resize(width, height int) {
	e.viewWidth, e.viewHeight = max(1, width), max(1, height-1)
	e.ensureVisible()
}

func (e *Editor) ensureVisible() {
	e.Cursor.X = min(max(e.Cursor.X, 0), e.Doc.Width-1)
	e.Cursor.Y = min(max(e.Cursor.Y, 0), e.Doc.Height-1)
	if e.Cursor.X < e.Offset.X {
		e.Offset.X = e.Cursor.X
	}
	if e.Cursor.Y < e.Offset.Y {
		e.Offset.Y = e.Cursor.Y
	}
	if e.Cursor.X >= e.Offset.X+e.viewWidth {
		e.Offset.X = e.Cursor.X - e.viewWidth + 1
	}
	if e.Cursor.Y >= e.Offset.Y+e.viewHeight {
		e.Offset.Y = e.Cursor.Y - e.viewHeight + 1
	}
	e.Offset.X = min(max(e.Offset.X, 0), max(0, e.Doc.Width-e.viewWidth))
	e.Offset.Y = min(max(e.Offset.Y, 0), max(0, e.Doc.Height-e.viewHeight))
}

func (e *Editor) Move(dx, dy int) {
	old := e.Cursor
	e.Cursor.X += dx
	e.Cursor.Y += dy
	e.ensureVisible()
	if old == e.Cursor {
		return
	}
	if e.Tool == ToolOval && e.Phase == 2 {
		// Preserve the exact initial circle even when its perpendicular handle
		// falls between terminal cells. Movement adds integer cell displacements.
		e.radius2.X += float64(e.Cursor.X - old.X)
		e.radius2.Y += float64(e.Cursor.Y - old.Y)
	}
	if e.Tool == ToolPen && e.PenDown {
		e.paintPen()
	}
}

func (e *Editor) brush() *canvas.Paint {
	if e.Tool == ToolText {
		return &e.Text
	}
	if e.Tool == ToolPen {
		return &e.Pen
	}
	if (e.Tool == ToolRectangle && e.Phase == 2) || (e.Tool == ToolOval && e.Phase == 3) {
		return &e.Fill
	}
	return &e.Outline
}

func (e *Editor) SwitchTool(tool Tool) {
	e.FinishStroke()
	e.Phase, e.Palette = 0, 0
	e.PenDown, e.Toolbar, e.Help = false, false, false
	e.Tool = tool
	b := e.brush()
	b.FG, b.BG = e.Text.FG, e.Text.BG
}

func (e *Editor) cancel() {
	e.Phase = 0
	e.PenDown = false
	e.FinishStroke()
}

func (e *Editor) shape() geometry.Shape {
	w, h := e.Doc.Width, e.Doc.Height
	switch e.Tool {
	case ToolLine:
		return geometry.Line(e.anchor, e.Cursor, e.Doc.Aspect, w, h)
	case ToolRectangle:
		b := e.Cursor
		if e.Phase == 2 {
			b = e.corner
		}
		return geometry.Rectangle(e.anchor, b, w, h)
	case ToolOval:
		u, v := e.radius1, e.radius2
		if e.Phase == 1 {
			u = geometry.Vector{X: float64(e.Cursor.X - e.anchor.X), Y: float64(e.Cursor.Y - e.anchor.Y)}
			v = geometry.Perpendicular(u, e.Doc.Aspect)
		}
		return geometry.Ellipse(e.anchor, u, v, e.Doc.Aspect, w, h)
	}
	return geometry.Shape{}
}

// Preview returns replacement cells without touching the document or undo stack.
func (e *Editor) Preview() map[canvas.Point]canvas.Cell {
	if e.Phase == 0 || e.Tool == ToolText || e.Tool == ToolPen {
		return nil
	}
	s := e.shape()
	out := make(map[canvas.Point]canvas.Cell, len(s.Outline))
	if (e.Tool == ToolRectangle && e.Phase == 2) || (e.Tool == ToolOval && e.Phase == 3) {
		if c, paint := e.Fill.Cell(' '); paint {
			for _, p := range s.Interior {
				out[p] = c
			}
		}
	}
	// Boundary always wins over fill; transparent boundary leaves its original cells.
	for p, glyph := range s.Outline {
		delete(out, p)
		if c, paint := e.Outline.Cell(glyph); paint {
			out[p] = c
		}
	}
	return out
}

func (e *Editor) Enter() {
	switch e.Tool {
	case ToolText:
		e.Cursor.X = 0
		e.Move(0, 1)
	case ToolPen:
		e.PenDown = !e.PenDown
		if e.PenDown {
			e.stroke = make(map[canvas.Point]change)
			e.paintPen()
		} else {
			e.FinishStroke()
		}
	case ToolLine:
		if e.Phase == 0 {
			e.anchor, e.Phase = e.Cursor, 1
		} else {
			e.commit(e.Preview())
			e.Phase = 0
		}
	case ToolRectangle:
		switch e.Phase {
		case 0:
			e.anchor, e.Phase = e.Cursor, 1
			e.fillStarted = false
		case 1:
			e.corner, e.Phase = e.Cursor, 2
			e.startFill()
		case 2:
			e.commit(e.Preview())
			e.Phase = 0
		}
	case ToolOval:
		switch e.Phase {
		case 0:
			e.anchor, e.Phase = e.Cursor, 1
			e.fillStarted = false
		case 1:
			e.radius1 = geometry.Vector{X: float64(e.Cursor.X - e.anchor.X), Y: float64(e.Cursor.Y - e.anchor.Y)}
			e.radius2 = geometry.Perpendicular(e.radius1, e.Doc.Aspect)
			e.Cursor = canvas.Point{X: e.anchor.X + int(math.Round(e.radius2.X)), Y: e.anchor.Y + int(math.Round(e.radius2.Y))}
			e.ensureVisible()
			e.secondStart, e.Phase = e.Cursor, 2
		case 2:
			e.Phase = 3
			e.startFill()
		case 3:
			e.commit(e.Preview())
			e.Phase = 0
		}
	}
}

func (e *Editor) startFill() {
	if !e.fillStarted {
		e.Fill.FG, e.Fill.BG = e.Text.FG, e.Text.BG
		e.fillStarted = true
	}
}

func (e *Editor) Backspace() {
	if e.Tool == ToolText {
		if e.Cursor.X > 0 {
			e.Move(-1, 0)
			e.commit(map[canvas.Point]canvas.Cell{e.Cursor: canvas.Blank()})
		}
		return
	}
	if e.Phase == 0 {
		return
	}
	if e.Tool == ToolOval {
		if e.Phase == 2 {
			e.Cursor = canvas.Point{X: e.anchor.X + int(e.radius1.X), Y: e.anchor.Y + int(e.radius1.Y)}
		} else if e.Phase == 3 {
			// The fill phase may move the cursor without changing the radius.
			e.Cursor = canvas.Point{X: e.secondStart.X + int(math.Round(e.radius2.X-geometry.Perpendicular(e.radius1, e.Doc.Aspect).X)), Y: e.secondStart.Y + int(math.Round(e.radius2.Y-geometry.Perpendicular(e.radius1, e.Doc.Aspect).Y))}
		}
	} else if e.Tool == ToolRectangle && e.Phase == 2 {
		e.Cursor = e.corner
	}
	e.Phase--
	if e.Phase == 0 {
		e.Cursor = e.anchor
	}
	e.ensureVisible()
}

func cycle(p *canvas.Paint, fill bool) {
	if fill {
		switch {
		case p.Mode == canvas.PaintTransparent:
			p.Mode, p.Glyph = canvas.PaintGlyph, ' '
		case p.Mode == canvas.PaintGlyph && p.Glyph == ' ':
			p.Mode = canvas.PaintDelete
		default:
			p.Mode = canvas.PaintTransparent
		}
		return
	}
	switch {
	case p.Mode == canvas.PaintAuto:
		p.Mode, p.Glyph = canvas.PaintGlyph, ' '
	case p.Mode == canvas.PaintGlyph && p.Glyph == ' ':
		p.Mode = canvas.PaintTransparent
	case p.Mode == canvas.PaintTransparent:
		p.Mode = canvas.PaintDelete
	case p.Mode == canvas.PaintDelete:
		p.Mode = canvas.PaintAuto
	default:
		p.Mode, p.Glyph = canvas.PaintGlyph, ' '
	}
}

func (e *Editor) Type(r rune) {
	if r < 32 || r > 126 {
		e.Message = "Use printable ASCII characters"
		return
	}
	if e.Tool == ToolText {
		c := canvas.Cell{Rune: r, FG: e.Text.FG, BG: e.Text.BG}
		e.commit(map[canvas.Point]canvas.Cell{e.Cursor: c})
		e.Move(1, 0)
		return
	}
	b := e.brush()
	if e.Tool == ToolPen {
		b.Mode, b.Glyph = canvas.PaintGlyph, r
		return
	}
	if r == ' ' {
		cycle(b, b == &e.Fill)
	} else {
		b.Mode, b.Glyph = canvas.PaintGlyph, r
	}
}

func (e *Editor) SetColor(color canvas.Color) {
	b := e.brush()
	if e.Palette == 2 {
		b.BG, e.Text.BG = color, color
	} else {
		b.FG, e.Text.FG = color, color
	}
	e.Palette = 0
}

func (e *Editor) TogglePalette() {
	if e.Palette == 1 {
		e.Palette = 2
	} else {
		e.Palette = 1
	}
	e.Toolbar, e.Help = false, false
	selected := e.brush().FG
	if e.Palette == 2 {
		selected = e.brush().BG
	}
	for i, c := range canvas.Colors {
		if c == selected {
			e.PaletteIndex = i
		}
	}
}

func (e *Editor) commit(cells map[canvas.Point]canvas.Cell) {
	t := make(transaction, 0, len(cells))
	for p, c := range cells {
		before := e.Doc.At(p)
		if e.Doc.InBounds(p) && c != before {
			e.Doc.Set(p, c)
			t = append(t, change{p, before, c})
		}
	}
	if len(t) > 0 {
		e.push(t)
		e.Revision++
	}
}

func (e *Editor) push(t transaction) {
	e.undo, e.redo = append(e.undo, t), nil
	// Keep history bounded while retaining at least the latest whole operation.
	n := 0
	for i := len(e.undo) - 1; i >= 0; i-- {
		n += len(e.undo[i])
		if i < len(e.undo)-1 && (n > 2_000_000 || len(e.undo)-i > 1000) {
			e.undo = e.undo[i+1:]
			break
		}
	}
}

func (e *Editor) paintPen() {
	c, paint := e.Pen.Cell('*')
	if !paint || c == e.Doc.At(e.Cursor) {
		return
	}
	if e.stroke == nil {
		e.stroke = make(map[canvas.Point]change)
	}
	d, exists := e.stroke[e.Cursor]
	if !exists {
		d = change{Point: e.Cursor, Before: e.Doc.At(e.Cursor)}
	}
	d.After = c
	e.stroke[e.Cursor] = d
	e.Doc.Set(e.Cursor, c)
	e.redo = nil
	e.Revision++
}

func (e *Editor) FinishStroke() {
	if len(e.stroke) > 0 {
		t := make(transaction, 0, len(e.stroke))
		for _, c := range e.stroke {
			if c.Before != c.After {
				t = append(t, c)
			}
		}
		if len(t) > 0 {
			e.push(t)
		}
	}
	e.stroke = nil
}

func (e *Editor) Undo() {
	if e.Phase > 0 {
		e.cancel()
		return
	}
	e.PenDown = false
	e.FinishStroke()
	if len(e.undo) == 0 {
		return
	}
	t := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for _, c := range t {
		e.Doc.Set(c.Point, c.Before)
	}
	e.redo = append(e.redo, t)
	e.Revision++
}

func (e *Editor) Redo() {
	e.cancel()
	if len(e.redo) == 0 {
		return
	}
	t := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, c := range t {
		e.Doc.Set(c.Point, c.After)
	}
	e.undo = append(e.undo, t)
	e.Revision++
}

func control(key *tcell.EventKey) string {
	if key.Key() >= tcell.KeyCtrlA && key.Key() <= tcell.KeyCtrlZ {
		// Enter, Tab, Backspace, and Escape are normal editing keys in legacy input.
		switch key.Key() {
		case tcell.KeyEnter, tcell.KeyTab, tcell.KeyBackspace, tcell.KeyEsc:
			return ""
		}
		return string(rune('a' + key.Key() - tcell.KeyCtrlA))
	}
	if key.Key() == tcell.KeyRune && key.Modifiers()&tcell.ModCtrl != 0 {
		return strings.ToLower(key.Str())
	}
	return ""
}

func (e *Editor) HandleKey(key *tcell.EventKey, now time.Time) Action {
	if !key.Pressed() {
		return ActionNone
	}
	ctrl := control(key)
	e.Message = ""
	if ctrl == "c" || ctrl == "q" || ctrl == "d" {
		if e.quitKey == ctrl && now.Before(e.quitUntil) {
			e.quitKey = ""
			return ActionQuit
		}
		e.quitKey, e.quitUntil = ctrl, now.Add(2*time.Second)
		e.Message = "Press Ctrl+" + strings.ToUpper(ctrl) + " again to save and quit"
		return ActionNone
	}
	e.quitKey = ""
	if ctrl != "" {
		switch ctrl {
		case "s":
			return ActionSave
		case "z":
			e.Undo()
		case "y":
			e.Redo()
		case "p":
			e.TogglePalette()
		case "l":
			return ActionRefresh
		default:
			e.Message = "Unused Ctrl+" + strings.ToUpper(ctrl)
		}
		return ActionNone
	}
	if key.Key() == tcell.KeyF7 {
		e.Help = !e.Help
		e.HelpScroll = 0
		e.Palette = 0
		e.Toolbar = false
		return ActionNone
	}
	if key.Key() == tcell.KeyF6 {
		e.TogglePalette()
		return ActionNone
	}
	if key.Key() >= tcell.KeyF1 && key.Key() <= tcell.KeyF5 {
		e.SwitchTool(Tool(key.Key() - tcell.KeyF1))
		return ActionNone
	}
	if e.Help {
		switch key.Key() {
		case tcell.KeyEsc:
			e.Help = false
		case tcell.KeyUp:
			e.HelpScroll = max(0, e.HelpScroll-1)
		case tcell.KeyDown:
			e.HelpScroll++
		case tcell.KeyPgUp:
			e.HelpScroll = max(0, e.HelpScroll-max(1, e.viewHeight-2))
		case tcell.KeyPgDn:
			e.HelpScroll += max(1, e.viewHeight-2)
		case tcell.KeyHome:
			e.HelpScroll = 0
		}
		return ActionNone
	}
	if e.Palette > 0 {
		switch key.Key() {
		case tcell.KeyEsc:
			e.Palette = 0
		case tcell.KeyTab:
			e.TogglePalette()
		case tcell.KeyLeft, tcell.KeyUp:
			e.PaletteIndex = (e.PaletteIndex + 9) % 10
		case tcell.KeyRight, tcell.KeyDown:
			e.PaletteIndex = (e.PaletteIndex + 1) % 10
		case tcell.KeyEnter:
			e.SetColor(canvas.Colors[e.PaletteIndex])
		case tcell.KeyRune:
			if len(key.Str()) == 1 && key.Str()[0] >= '0' && key.Str()[0] <= '9' {
				e.SetColor(canvas.Colors[(int(key.Str()[0]-'0')+9)%10])
			}
		}
		return ActionNone
	}
	if e.Toolbar {
		switch key.Key() {
		case tcell.KeyEsc, tcell.KeyTab:
			e.Toolbar = false
		case tcell.KeyLeft, tcell.KeyUp:
			e.ToolbarIndex = (e.ToolbarIndex + 6) % 7
		case tcell.KeyRight, tcell.KeyDown:
			e.ToolbarIndex = (e.ToolbarIndex + 1) % 7
		case tcell.KeyEnter:
			if e.ToolbarIndex < 5 {
				e.SwitchTool(Tool(e.ToolbarIndex))
			} else if e.ToolbarIndex == 5 {
				e.TogglePalette()
			} else {
				e.Help = true
				e.HelpScroll = 0
				e.Toolbar = false
			}
		}
		return ActionNone
	}
	if key.Modifiers()&tcell.ModAlt != 0 {
		return ActionNone
	}
	switch key.Key() {
	case tcell.KeyTab:
		e.Toolbar = true
		e.ToolbarIndex = int(e.Tool)
	case tcell.KeyLeft:
		e.Move(-1, 0)
	case tcell.KeyRight:
		e.Move(1, 0)
	case tcell.KeyUp:
		e.Move(0, -1)
	case tcell.KeyDown:
		e.Move(0, 1)
	case tcell.KeyHome:
		e.Move(-e.Cursor.X, 0)
	case tcell.KeyEnd:
		e.Move(e.Doc.Width-1-e.Cursor.X, 0)
	case tcell.KeyPgUp:
		e.Move(0, -e.viewHeight)
	case tcell.KeyPgDn:
		e.Move(0, e.viewHeight)
	case tcell.KeyEnter:
		e.Enter()
	case tcell.KeyEsc:
		e.cancel()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.Backspace()
	case tcell.KeyDelete:
		if e.Tool == ToolText {
			e.commit(map[canvas.Point]canvas.Cell{e.Cursor: canvas.Blank()})
		} else {
			e.brush().Mode = canvas.PaintDelete
		}
	case tcell.KeyRune:
		if len(key.Str()) == 1 {
			e.Type(rune(key.Str()[0]))
		} else {
			e.Message = "Use printable ASCII characters"
		}
	}
	return ActionNone
}

// Tick expires the accidental-quit guard without waiting for another key.
func (e *Editor) Tick(now time.Time) bool {
	if e.quitKey != "" && !now.Before(e.quitUntil) {
		prompt := "Press Ctrl+" + strings.ToUpper(e.quitKey) + " again to save and quit"
		e.quitKey = ""
		if e.Message == prompt {
			e.Message = ""
			return true
		}
	}
	return false
}

func paintName(p canvas.Paint) string {
	switch p.Mode {
	case canvas.PaintAuto:
		return "AUTO"
	case canvas.PaintTransparent:
		return "TRANSPARENT"
	case canvas.PaintDelete:
		return "DELETE"
	default:
		if p.Glyph == ' ' {
			return "SPACE"
		}
		return fmt.Sprintf("%c", p.Glyph)
	}
}

func (e *Editor) Status() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Toolbar {
		return "Arrows: choose | Enter"
	}
	if e.Tool == ToolText {
		return fmt.Sprintf("%s/%s", e.Text.FG, e.Text.BG)
	}
	if e.Tool == ToolPen {
		position := "UP"
		if e.PenDown {
			position = "DOWN"
		}
		return "Pen " + position + ": " + paintName(e.Pen)
	}
	label := "Outline"
	if e.brush() == &e.Fill {
		label = "Fill"
	}
	return label + ": " + paintName(*e.brush())
}
