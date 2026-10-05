// Package ui renders the editor without modifying its document or operation.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/clipperhouse/displaywidth"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"

	"tdoodle/internal/canvas"
	"tdoodle/internal/editor"
)

// Screen contains the display operations used by the renderer.  Keeping input
// outside this interface makes rendering testable without an actual terminal.
type Screen interface {
	Size() (int, int)
	Put(int, int, string, tcell.Style) (string, int)
	ShowCursor(int, int)
	HideCursor()
	Show()
	Colors() int
}

// Render redraws the viewport and its single status row. Uncommitted operations
// are composited over the document, so previews never become permanent here.
func Render(screen Screen, e *editor.Editor) {
	w, h := screen.Size()
	if w <= 0 || h <= 0 {
		screen.HideCursor()
		screen.Show()
		return
	}
	colors := screen.Colors()
	linux := os.Getenv("TERM") == "linux"
	blank := canvas.Blank()
	preview := e.Preview()
	for y := 0; y < h-1; y++ {
		for x := 0; x < w; x++ {
			point := canvas.Point{X: x + e.Offset.X, Y: y + e.Offset.Y}
			cell := blank
			if e.Doc != nil {
				cell = e.Doc.At(point)
			}
			if overlay, ok := preview[point]; ok {
				cell = overlay
			}
			glyph := cell.Rune
			if glyph == 0 {
				glyph = ' '
			}
			style := cellStyle(cell, colors, linux)
			if point == e.Cursor && !e.Help && e.Palette == 0 && !e.Toolbar {
				style = style.Reverse(true)
			}
			screen.Put(x, y, string(glyph), style)
		}
	}
	base := tcell.StyleDefault.Foreground(color.Silver).Background(color.Black)
	for x := 0; x < w; x++ {
		screen.Put(x, h-1, " ", base)
	}
	if e.Palette != 0 {
		renderPalette(screen, e, w, h-1, colors, linux)
	} else {
		renderStatus(screen, e, w, h-1, base)
	}
	if e.Help {
		renderHelp(screen, e.HelpScroll, w, h-1, base)
	}
	cx, cy := e.Cursor.X-e.Offset.X, e.Cursor.Y-e.Offset.Y
	if !e.Help && e.Palette == 0 && !e.Toolbar && cx >= 0 && cx < w && cy >= 0 && cy < h-1 {
		screen.ShowCursor(cx, cy)
	} else {
		screen.HideCursor()
	}
	screen.Show()
}

var toolNames = []string{"Text", "Oval", "Rectangle", "Line", "Pen", "Colors", "Help"}
var toolShort = []string{"Text", "Oval", "Rect", "Line", "Pen", "Color", "Help"}
var toolTiny = []string{"T", "O", "R", "L", "P", "C", "?"}

func toolbarLabels(kind int) []string {
	labels := make([]string, len(toolNames))
	for i := range labels {
		switch kind {
		case 0:
			labels[i] = fmt.Sprintf("%s [F%d]", toolNames[i], i+1)
		case 1:
			labels[i] = fmt.Sprintf("%s F%d", toolShort[i], i+1)
		default:
			labels[i] = fmt.Sprintf("%s%d", toolTiny[i], i+1)
		}
	}
	return labels
}

func renderStatus(screen Screen, e *editor.Editor, w, y int, base tcell.Style) {
	active := int(e.Tool)
	if active < 0 || active > 4 {
		active = 0
	}
	if e.Help {
		active = 6
	}
	minimum := fmt.Sprintf("%s%d", toolTiny[active], active+1)
	right := e.Status()
	if e.Help && e.Message == "" {
		right = "Arrows/PgUp/PgDn: scroll | Esc: close"
	}
	// Reserve the active tool even when a long prompt needs most of the row.
	rightWidth := min(displaywidth.String(right), max(0, w-len(minimum)-1))
	if w <= len(minimum) {
		putText(screen, 0, y, minimum, w, base.Reverse(true).Bold(true))
		return
	}
	leftWidth := w
	if rightWidth > 0 {
		leftWidth -= rightWidth + 1
	}
	labels := toolbarLabels(0)
	if len(strings.Join(labels, " | ")) > leftWidth {
		labels = toolbarLabels(1)
	}
	if len(strings.Join(labels, " | ")) > leftWidth {
		labels = toolbarLabels(2)
	}
	if len(strings.Join(labels, " | ")) > leftWidth {
		index := active
		if e.Toolbar && e.ToolbarIndex >= 0 && e.ToolbarIndex < len(labels) {
			index = e.ToolbarIndex
		}
		label := fmt.Sprintf("%s F%d", toolShort[index], index+1)
		if len(label) > leftWidth {
			label = labels[index]
		}
		style := base.Reverse(true).Bold(true)
		putText(screen, 0, y, label, leftWidth, style)
	} else {
		x := 0
		for i, label := range labels {
			if i > 0 {
				x += putText(screen, x, y, " | ", leftWidth-x, base)
			}
			style := base
			if i == active {
				style = style.Bold(true).Reverse(true)
			}
			if e.Toolbar && i == e.ToolbarIndex {
				style = style.Underline(true).Reverse(true)
			}
			x += putText(screen, x, y, label, leftWidth-x, style)
		}
	}
	if rightWidth > 0 {
		style := base
		if e.Message != "" {
			style = style.Bold(true)
		}
		putText(screen, w-rightWidth, y, right, rightWidth, style)
	}
}

func renderPalette(screen Screen, e *editor.Editor, w, y, colors int, linux bool) {
	base := tcell.StyleDefault.Foreground(color.Silver).Background(color.Black)
	name, short := "Foreground", "FG"
	paint := currentPaint(e)
	selected := paint.FG
	if e.Palette == 2 {
		name, short = "Background", "BG"
		selected = paint.BG
	}
	approx := colors < 256 || (linux && e.Palette == 2)
	prefix := name + ": "
	if approx {
		prefix = name + " (approx): "
	}
	labels := make([]string, len(canvas.Colors))
	for i, c := range canvas.Colors {
		name := string(c)
		labels[i] = paletteLabel(i, fmt.Sprintf("%d %s", (i+1)%10, strings.ToUpper(name[:1])+name[1:]), e.PaletteIndex)
	}
	if len(prefix)+len(strings.Join(labels, " ")) > w {
		prefix = short + ": "
		if approx {
			prefix = short + "~: "
		}
		abbreviated := []string{"Red", "Org", "Yel", "Grn", "Blu", "Vio", "Gry", "Brn", "Wht", "Blk"}
		for i := range labels {
			labels[i] = paletteLabel(i, fmt.Sprintf("%d%s", (i+1)%10, abbreviated[i]), e.PaletteIndex)
		}
	}
	if len(prefix)+len(strings.Join(labels, " ")) > w {
		for i := range labels {
			labels[i] = paletteLabel(i, fmt.Sprint((i+1)%10), e.PaletteIndex)
		}
	}
	if len(prefix)+len(strings.Join(labels, " ")) > w {
		prefix = short
		if approx {
			prefix += "~"
		}
		prefix += " "
	}
	if w <= len(prefix) {
		i := max(0, min(e.PaletteIndex, len(canvas.Colors)-1))
		putText(screen, 0, y, fmt.Sprint((i+1)%10), w, swatchStyle(canvas.Colors[i], e.Palette, colors, linux).Underline(true).Bold(true))
		return
	}
	x := putText(screen, 0, y, prefix, w, base)
	// Very narrow displays show the current arrow selection and its name.
	if x+len(strings.Join(labels, " ")) > w {
		i := max(0, min(e.PaletteIndex, len(canvas.Colors)-1))
		label := fmt.Sprintf("%d %s", (i+1)%10, string(canvas.Colors[i]))
		putText(screen, x, y, label, w-x, swatchStyle(canvas.Colors[i], e.Palette, colors, linux).Underline(true).Bold(true))
		return
	}
	for i, c := range canvas.Colors {
		if i > 0 {
			x += putText(screen, x, y, " ", w-x, base)
		}
		style := swatchStyle(c, e.Palette, colors, linux)
		if c == selected {
			style = style.Bold(true)
		}
		if i == e.PaletteIndex {
			style = style.Underline(true)
		}
		x += putText(screen, x, y, labels[i], w-x, style)
	}
}

func paletteLabel(index int, label string, selected int) string {
	if index == selected {
		return "[" + label + "]"
	}
	return label
}

func currentPaint(e *editor.Editor) canvas.Paint {
	switch e.Tool {
	case editor.ToolText:
		return e.Text
	case editor.ToolPen:
		return e.Pen
	case editor.ToolOval:
		if e.Phase == 3 {
			return e.Fill
		}
	case editor.ToolRectangle:
		if e.Phase == 2 {
			return e.Fill
		}
	}
	return e.Outline
}

func swatchStyle(c canvas.Color, palette, colors int, linux bool) tcell.Style {
	if palette == 1 {
		fg := ColorFor(c, false, colors, linux)
		r, g, b := fg.RGB()
		bg := color.Black
		if 299*r+587*g+114*b < 60000 {
			bg = color.Silver
		}
		return tcell.StyleDefault.Foreground(fg).Background(bg)
	}
	bg := ColorFor(c, true, colors, linux)
	r, g, b := bg.RGB()
	fg := color.White
	if colors < 16 {
		fg = color.Silver
	}
	if 299*r+587*g+114*b > 100000 {
		fg = color.Black
	}
	return tcell.StyleDefault.Foreground(fg).Background(bg)
}

// putText writes only inside the requested span, advancing by displayed width.
func putText(screen Screen, x, y int, text string, width int, style tcell.Style) int {
	used := 0
	graphemes := displaywidth.StringGraphemes(text)
	for graphemes.Next() && used < width {
		cells := graphemes.Width()
		if cells <= 0 {
			continue
		}
		if cells > width-used {
			break
		}
		screen.Put(x+used, y, graphemes.Value(), style)
		used += cells
	}
	return used
}
