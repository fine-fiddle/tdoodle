package ui

import (
	"strings"

	"github.com/gdamore/tcell/v3"
)

var helpText = []string{
	"Up/Down or Page Up/Down scroll; Home starts; F8 / Esc closes this help.",
	"Arrows move. For shapes, press Enter at each ->; the last Enter draws.",
	"Status shows the current step and next Enter action; yellow PREVIEW means unfinished.",
	"Backspace goes back one shape step. Esc or another tool cancels the preview.",
	"",
	"F1 TEXT - Place characters anywhere.",
	"[Move cursor] --type--> [Paint + move right]",
	"",
	"F2 OVAL - Size a circle, then stretch it.",
	"[Center] -> [Circle size] -> [Stretch] -> [Fill] -> [Draw]",
	"",
	"F3 RECTANGLE - Pick two opposite corners.",
	"[Corner] -> [Opposite corner] -> [Fill] -> [Draw]",
	"",
	"F4 LINE - Connect two points.",
	"[Start] -> [End] -> [Draw]",
	"",
	"F5 PEN - Draw as you move with the pen down.",
	"[Pen up] <--Enter--> [Pen down: move to draw]",
	"",
	"F7 PICKER - Sample artwork with a separate cursor; drawing stays paused.",
	"[Drawing] --F7--> [Move sampler] --Enter / sample--> [Drawing]",
	"Copies character and both colors to the active outline/fill/pen brush.",
	"Text copies colors only. Samples canvas cells beneath previews; spaces stay literal.",
	"Esc or F7 cancels. Opening the picker ends a pen stroke and lifts the pen.",
	"",
	"KEYBOARD",
	"Type: paint text, or choose the shape/pen character (printable ASCII).",
	"Enter: text starts a new row; shapes advance; pen lowers/lifts.",
	"Backspace: erase previous text cell. Delete: erase cell / choose eraser.",
	"Esc: cancel unfinished shape, lift pen, or close picker/help/colors.",
	"Space: text/pen paint a blank; shapes cycle the shown brush mode.",
	"Auto = outline; Space = blank; Transparent = keep art; Delete = erase.",
	"F6 / Ctrl+P: colors. F6 or Tab switches foreground/background.",
	"1-9, 0 choose a color; arrows + Enter also choose; Esc closes.",
	"F7: open / cancel picker; arrows, Home/End, Page Up/Down move its cursor.",
	"Enter samples; Esc cancels. F8: open / close help.",
	"Tab: toolbar; arrows select; Enter activates; Esc returns.",
	"Home / End: row edges. Page Up / Down: move by a screen.",
	"Ctrl+Z / Ctrl+Y: undo / redo. Ctrl+S: save. Ctrl+L: refresh.",
	"Ctrl+C twice within 2 seconds: save and quit. Ctrl+Q / Ctrl+D also work.",
	"",
	"GNU GPL version 3; no warranty. See LICENSE.",
}

// HelpRows gives the number of body rows after wrapping to a terminal width.
func HelpRows(width int) int { return len(wrappedHelp(max(1, width))) }

func renderHelp(screen Screen, scroll, width, height int, style tcell.Style) {
	if height <= 0 {
		return
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			screen.Put(x, y, " ", style)
		}
	}
	putText(screen, 0, 0, "tDoodle help", width, style.Bold(true).Underline(true))
	if height == 1 {
		return
	}
	lines := wrappedHelp(width)
	rows := height - 1
	scroll = max(0, min(scroll, max(0, len(lines)-rows)))
	for y := 0; y < rows && scroll+y < len(lines); y++ {
		putText(screen, 0, y+1, lines[scroll+y], width, style)
	}
}

func wrappedHelp(width int) []string {
	width = max(1, width)
	var lines []string
	for _, paragraph := range helpText {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line != "" && len(line)+len(word)+1 > width {
				lines = append(lines, line)
				line = ""
			}
			// Long words (including the license URL) still fit narrow terminals.
			for len(word) > width {
				if line != "" {
					lines = append(lines, line)
					line = ""
				}
				lines = append(lines, word[:width])
				word = word[width:]
			}
			if word == "" {
				continue
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
