package ui

import (
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"

	"tdoodle/internal/canvas"
)

var rgbColors = map[canvas.Color]color.Color{
	canvas.Red: color.NewRGBColor(230, 80, 80), canvas.Orange: color.NewRGBColor(245, 158, 66),
	canvas.Yellow: color.NewRGBColor(242, 207, 74), canvas.Green: color.NewRGBColor(102, 191, 115),
	canvas.Blue: color.NewRGBColor(89, 142, 222), canvas.Violet: color.NewRGBColor(183, 123, 224),
	canvas.Gray: color.NewRGBColor(144, 144, 144), canvas.Brown: color.NewRGBColor(149, 101, 69),
	canvas.White: color.NewRGBColor(244, 244, 244), canvas.Black: color.NewRGBColor(16, 16, 16),
}

var ansi16Colors = map[canvas.Color]color.Color{
	canvas.Red: color.Red, canvas.Orange: color.Yellow,
	canvas.Yellow: color.Yellow, canvas.Green: color.Lime,
	canvas.Blue: color.Blue, canvas.Violet: color.Fuchsia,
	canvas.Gray: color.Gray, canvas.Brown: color.Olive,
	canvas.White: color.White, canvas.Black: color.Black,
}

var ansi8Colors = map[canvas.Color]color.Color{
	canvas.Red: color.Maroon, canvas.Orange: color.Olive,
	canvas.Yellow: color.Olive, canvas.Green: color.Green,
	canvas.Blue: color.Navy, canvas.Violet: color.Purple,
	canvas.Gray: color.Silver, canvas.Brown: color.Olive,
	canvas.White: color.Silver, canvas.Black: color.Black,
}

// ColorFor maps a document's logical color to the current display.  Logical
// colors stay intact in saved artwork even where the terminal approximates them.
func ColorFor(c canvas.Color, background bool, colors int, linux bool) color.Color {
	if colors <= 0 {
		return color.Default
	}
	if linux && background && colors > 8 {
		colors = 8
	}
	if colors >= 256 {
		if value, ok := rgbColors[c]; ok {
			return value
		}
		return color.White
	}
	if colors < 8 {
		if c == canvas.Black {
			return color.Black
		}
		return color.Silver
	}
	if colors >= 16 {
		if result, ok := ansi16Colors[c]; ok {
			return result
		}
		return color.White
	}
	if result, ok := ansi8Colors[c]; ok {
		return result
	}
	return color.Silver
}

func cellStyle(c canvas.Cell, colors int, linux bool) tcell.Style {
	return tcell.StyleDefault.
		Foreground(ColorFor(c.FG, false, colors, linux)).
		Background(ColorFor(c.BG, true, colors, linux))
}
