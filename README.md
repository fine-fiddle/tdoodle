# tDoodle

A terminal drawing tool, written in Go with tcell.

## Compile

Requires Go 1.25 or later and an interactive terminal.

```sh
go build -o tdoodle .
./tdoodle castle.tdoodle
```

## Run

tdoodle [filename] 
Open a .tdoodle drawing for editing. 
A missing filename starts a blank drawing with the date time as filename

##  Drawing

The F-keys choose your brush. 
Return progresses through the phases of your brush's drawing, and backspace regresses.

| Key | Action |
| --- | --- |
| Arrows | Move the cursor or current shape handle |
| F1 | Text: type to paint; Enter starts the next row; no automatic wrapping |
| F2 | Oval: Enter at center, first radius, second radius, then fill |
| F3 | Rectangle: Enter at first corner, opposite corner, then fill |
| F4 | Line: Enter at start and end |
| F5 | Pen: Enter toggles pen down/up; move to paint; type to change character |
| F6 | Open foreground colors; press again for background colors |
| F7 | Picker: move sampler; Enter copies character/colors; Escape cancels |
| F8 | Open/close help; arrows and Page Up/Down scroll |
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
The right side of the status row shows the current step and what Enter does
next. An unfinished shape is marked `PREVIEW` in bold yellow until it is drawn
or canceled. Narrow terminals keep the preview warning and next action ahead
of brush details; very narrow terminals show just the warning. Toolbar and help
show a paused-preview warning while the shape remains unfinished.
Escape and switching tools cancel the preview. A completed shape or pen stroke
is one undo step. 

F7 opens a separate picker cursor in any drawing phase. Move to a source cell
with arrows, Home/End, or Page Up/Down; Enter copies its character and both colors
to the active outline, fill, or pen brush, then returns to the drawing cursor.
Text copies colors only. The picker reads canvas cells beneath previews; a
sampled space stays a literal space. Escape or F7 cancels. Entering the picker
ends an active pen stroke and lifts the pen. Help is the last toolbar item, on F8.

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
