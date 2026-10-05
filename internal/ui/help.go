package ui

import (
	"strings"

	"github.com/gdamore/tcell/v3"
)

var helpText = []string{
	"Move with the arrow keys. The canvas keeps its size when the terminal is resized; the view follows the cursor.",
	"Home/End go to the left/right canvas edge. Page Up/Down move by a screen. Ctrl+L refreshes the screen.",
	"F1 Text | F2 Oval | F3 Rectangle | F4 Line | F5 Pen | F6 Colors | F7 Help",
	"Tab opens the toolbar. Arrows choose a tool; Enter activates it. Escape returns to drawing.",
	"",
	"Text: type printable ASCII characters at the cursor. Enter starts the next row. Backspace erases the previous cell; Delete erases this cell. Text does not wrap.",
	"",
	"Shapes show a live preview until the final Enter. Backspace goes back one phase. Escape or a different tool cancels the unfinished shape.",
	"Line: Enter at the start, move to the end, then Enter to draw.",
	"Rectangle: Enter at one corner, move to the opposite corner, Enter, choose the fill, then Enter to draw.",
	"Oval: Enter at the center, move to set the first radius, Enter, adjust the second radius, Enter, choose the fill, then Enter to draw. The second radius can point in any direction.",
	"The oval begins as a circle adjusted for terminal cell proportions. Its radius handles can produce a line or a point.",
	"",
	"Type a character during a shape to use it for the current outline or fill. Space changes the brush mode shown in the status bar.",
	"Outline Space cycle: AUTO -> SPACE -> TRANSPARENT -> DELETE -> AUTO. Space after a typed outline character selects SPACE.",
	"Fill Space cycle: TRANSPARENT -> SPACE -> DELETE -> TRANSPARENT. Space after a typed fill character selects TRANSPARENT.",
	"AUTO chooses ASCII outline characters. SPACE paints a blank in the selected colors. TRANSPARENT leaves the existing artwork. DELETE clears to white on black. The Delete key selects DELETE immediately.",
	"",
	"Pen: Enter lowers or lifts the pen. Move to draw; type a character to change its mark. Space is a literal blank. Delete selects erasing. A whole stroke is one undo step.",
	"",
	"F6 (or Ctrl+P) opens foreground colors; F6 again switches to background. Press 1 through 9 or 0 to choose immediately. Arrows and Enter also work. Escape closes colors.",
	"Colors affect the current brush. Colors marked ~ or approx use the terminal's nearest palette; some colors look the same. Linux console backgrounds use eight colors. Saved files retain the chosen logical colors.",
	"",
	"Ctrl+Z undoes; Ctrl+Y redoes. Ctrl+S saves. Press Ctrl+C twice within two seconds to save and quit (Ctrl+Q or Ctrl+D also work). A save error keeps the drawing open.",
	"Help and colors preserve unfinished shapes. Use arrows or Page Up/Down to scroll this help, Home for its beginning, and F7 or Escape to close it.",
	"",
	"tDoodle is free software under the GNU GPL version 3. It comes with absolutely no warranty. See LICENSE and https://www.gnu.org/licenses/gpl-3.0.html",
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
