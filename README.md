# tDoodle

A keyboard drawing program for kids, written in Go with tcell. Paint text, lines,
rectangles, ovals, and freehand pen strokes on an ASCII canvas.

## Run

Requires Go 1.25 or later and an interactive terminal.

```sh
go build -o tdoodle .
./tdoodle castle.tdoodle
```

An existing drawing opens for editing. A missing filename starts a blank drawing
and becomes the save target. Unreadable or malformed existing drawings produce
an error before entering the editor.

```sh
./tdoodle
```

Without a filename, tDoodle uses a name such as
`tdoodle-20261004T213000.123456789.tdoodle` in the current directory.

Press **Ctrl+C twice within two seconds** to save and quit. **Ctrl+S** saves while
you keep drawing. A failed save displays an error and leaves the editor open.
Quit saves completed artwork; an unfinished shape is still a preview.

## Drawing

The canvas starts at the terminal width and height minus one status row. A
drawing keeps those dimensions when reopened or resized; the view scrolls to
follow the cursor. Characters overwrite cells without moving neighboring art.
Only printable ASCII is accepted.

| Key | Action |
| --- | --- |
| Arrows | Move the cursor or current shape handle |
| F1 | Text: type to paint; Enter starts the next row; no automatic wrapping |
| F2 | Oval: Enter at center, first radius, second radius, then fill |
| F3 | Rectangle: Enter at first corner, opposite corner, then fill |
| F4 | Line: Enter at start and end |
| F5 | Pen: Enter toggles pen down/up; move to paint; type to change character |
| F6 | Open foreground colors; press again for background colors |
| F7 | Open/close help; arrows and Page Up/Down scroll |
| Tab | Focus toolbar; arrows select; Enter activates |
| Backspace | Text: move left and erase; shape: return to previous phase |
| Delete | Text: erase current cell; shape/pen: select erase brush |
| Escape | Cancel unfinished shape, lift pen, or close overlay |
| Ctrl+Z / Ctrl+Y | Undo / redo |
| Ctrl+S | Save |
| Ctrl+C twice | Save and quit (Ctrl+Q / Ctrl+D also work) |
| Home / End | Move to row edges |
| Page Up / Page Down | Move by a screen height |
| Ctrl+P / Ctrl+L | Open colors / refresh screen |

Shapes preview over existing artwork and commit only on the final Enter.
Escape and switching tools cancel the preview. A completed shape or pen stroke
is one undo step. Help and colors preserve an unfinished shape.

The oval starts as a circle corrected for rectangular terminal cells. Its second
radius initially preserves that circle, including radii between cells; moving
the handle stretches or skews the oval. The two radii need not be perpendicular.
Collinear radii make a line; zero radii make a point.

### Brushes

Typing a printable character selects it for the active outline or fill.
Repeated Space presses cycle modes, shown at the right of the status row:

```text
Outline: Automatic → Space → Transparent → Delete → Automatic
Fill:    Transparent → Space → Delete → Transparent
```

Space after a typed outline character selects a literal space; Space after a
typed fill character selects transparent fill. In text and pen tools, Space is
always a literal space.

- **Automatic** chooses ASCII outline glyphs from the slope.
- **Space** paints a space with the selected foreground/background colors.
- **Transparent** leaves destination cells unchanged.
- **Delete** restores the canonical blank cell: space, white foreground, black background.

Outline and fill styles are independent. Changing a fill color preserves the
outline color. Painting replaces destination cells; transparent regions preserve
earlier artwork. Fill comes from the mathematical shape, including when clipped
at canvas edges.

### Colors

F6 replaces the status row with a numbered palette:

`1 Red · 2 Orange · 3 Yellow · 4 Green · 5 Blue · 6 Violet · 7 Gray · 8 Brown · 9 White · 0 Black`

A number chooses its color and immediately returns to drawing. F6 or Tab toggles
foreground/background; arrows and Enter also select colors. Colors marked
`approx` or `~` use terminal approximations. Linux console backgrounds use eight
colors, so some choices can look alike. Files keep the logical color names.

## Saving and recovery

The native format is versioned JSON containing a `format: "tdoodle"` marker,
version, dimensions, terminal cell aspect, and row-major cells. Each cell stores
an ASCII rune number plus named foreground and background colors. Undo history
and unfinished shapes are session-only.

Saves write a temporary file beside the target, synchronize it, and atomically
rename it over the target. Existing file permissions are preserved. New files
are private to their owner. Saving to a symbolic link is refused.

Recovery copies are saved every 15 seconds after edits to
`<filename>.recovery`. Reopening a drawing automatically restores a valid newer
recovery copy. A successful normal save removes it. Corrupt recovery copies are
left in place, and a corrupt primary drawing causes startup to fail; a valid
recovery can be opened directly by its filename. For unnamed sessions, use the
timestamped recovery filename to reopen after a crash. Recovery retains completed
cells, including an active pen stroke, but cannot retain unfinished shape previews.

```sh
./tdoodle -aspect 2 -autosave 15s castle.tdoodle
./tdoodle -autosave 0 castle.tdoodle
```

`-aspect` is cell height divided by width (default `2`) for a new drawing. Existing
drawings use their saved aspect. Orderly SIGINT, SIGTERM, and SIGHUP attempt save
and quit; save failures keep the editor open. Abrupt termination or power loss
may lose changes since the last recovery copy.

## Development

```sh
go test ./...
go vet ./...
go test -race ./...
```

- `internal/canvas`: document, colors, paint modes, validation, atomic storage.
- `internal/geometry`: clipped line/rectangle/affine oval rasterization.
- `internal/editor`: input, tool phases, preview composition, undo/redo, viewport.
- `internal/ui`: single-row toolbar, color adaptation, help, terminal rendering.
- `main.go`: command line, terminal lifecycle, save/recovery scheduling.

Tests cover cancellation without changing artwork, grouped undo, all paint
modes, shape clipping and degenerate geometry, circle continuity, resize
preservation, file round-trips, malformed files, and failed save behavior.

## License

GNU GPL version 3; see [LICENSE](LICENSE). tDoodle comes with no warranty.
