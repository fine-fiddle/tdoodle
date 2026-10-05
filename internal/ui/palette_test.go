package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3/color"

	"tdoodle/internal/canvas"
)

func TestColorMappingsRespectTerminalLimits(t *testing.T) {
	for _, logical := range canvas.Colors {
		c := ColorFor(logical, false, 256, false)
		if !c.IsRGB() {
			t.Fatalf("rich terminal lost RGB mapping for %s", logical)
		}
		for _, colors := range []int{8, 16, 256} {
			bg := ColorFor(logical, true, colors, true)
			if bg < color.XTerm0 || bg > color.XTerm7 {
				t.Fatalf("Linux background %s mapped outside eight colors: %v", logical, bg)
			}
		}
		c = ColorFor(logical, false, 8, false)
		if c < color.XTerm0 || c > color.XTerm7 {
			t.Fatalf("eight-color foreground %s = %v", logical, c)
		}
	}
	if ColorFor(canvas.Red, false, 16, true) != color.Red {
		t.Fatal("Linux foreground should retain bright 16-color red")
	}
	if ColorFor(canvas.Red, true, 16, true) != color.Maroon {
		t.Fatal("Linux background should use normal eight-color red")
	}
	if ColorFor(canvas.Red, false, 0, false) != color.Default {
		t.Fatal("monochrome screen should use default colors")
	}
	if ColorFor(canvas.Orange, false, 256, false) == ColorFor(canvas.Yellow, false, 256, false) {
		t.Fatal("rich palette should distinguish orange and yellow")
	}
}
