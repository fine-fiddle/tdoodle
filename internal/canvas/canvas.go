// Package canvas holds tDoodle's terminal-independent drawing model.
package canvas

import (
	"fmt"
	"math"
)

const (
	CurrentVersion = 1
	MaxWidth       = 4096
	MaxHeight      = 4096
	MaxCells       = 4 * 1024 * 1024
	MinAspect      = 0.1
	MaxAspect      = 10.0
)

// Point identifies a cell, with the origin at the upper left.
type Point struct {
	X int
	Y int
}

// Color is a logical color. The terminal renderer supplies its closest match.
type Color string

const (
	Red    Color = "red"
	Orange Color = "orange"
	Yellow Color = "yellow"
	Green  Color = "green"
	Blue   Color = "blue"
	Violet Color = "violet"
	Gray   Color = "gray"
	Brown  Color = "brown"
	White  Color = "white"
	Black  Color = "black"
)

// Colors is the numbered palette order (1 through 9, then 0).
var Colors = []Color{Red, Orange, Yellow, Green, Blue, Violet, Gray, Brown, White, Black}

func (c Color) Valid() bool {
	switch c {
	case Red, Orange, Yellow, Green, Blue, Violet, Gray, Brown, White, Black:
		return true
	default:
		return false
	}
}

// Cell represents one printable ASCII character and its logical colors.
type Cell struct {
	Rune rune  `json:"rune"`
	FG   Color `json:"fg"`
	BG   Color `json:"bg"`
}

func Blank() Cell { return Cell{Rune: ' ', FG: White, BG: Black} }

func printable(r rune) bool { return r >= ' ' && r <= '~' }

func (c Cell) validate() error {
	if !printable(c.Rune) {
		return fmt.Errorf("character %U is not printable ASCII", c.Rune)
	}
	if !c.FG.Valid() {
		return fmt.Errorf("unknown foreground color %q", c.FG)
	}
	if !c.BG.Valid() {
		return fmt.Errorf("unknown background color %q", c.BG)
	}
	return nil
}

// Document retains its original dimensions when the terminal is resized.
// Aspect is the terminal cell height divided by its width.
type Document struct {
	Version int     `json:"version"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Aspect  float64 `json:"aspect"`
	Cells   []Cell  `json:"cells"`
}

func validateDimensions(w, h int, aspect float64) error {
	if w < 1 || w > MaxWidth || h < 1 || h > MaxHeight {
		return fmt.Errorf("canvas dimensions must be within 1..%d by 1..%d", MaxWidth, MaxHeight)
	}
	if w*h > MaxCells {
		return fmt.Errorf("canvas exceeds %d cells", MaxCells)
	}
	if math.IsNaN(aspect) || math.IsInf(aspect, 0) || aspect < MinAspect || aspect > MaxAspect {
		return fmt.Errorf("cell aspect must be finite and within %g..%g", MinAspect, MaxAspect)
	}
	return nil
}

func New(w, h int, aspect float64) (*Document, error) {
	if err := validateDimensions(w, h, aspect); err != nil {
		return nil, err
	}
	d := &Document{Version: CurrentVersion, Width: w, Height: h, Aspect: aspect, Cells: make([]Cell, w*h)}
	for i := range d.Cells {
		d.Cells[i] = Blank()
	}
	return d, nil
}

func (d *Document) InBounds(p Point) bool {
	return d != nil && p.X >= 0 && p.Y >= 0 && p.X < d.Width && p.Y < d.Height
}

func (d *Document) At(p Point) Cell {
	if !d.InBounds(p) {
		return Blank()
	}
	return d.Cells[p.Y*d.Width+p.X]
}

// Set writes an in-bounds cell and returns whether the point was in bounds.
// Call Validate before persisting manually constructed or modified documents.
func (d *Document) Set(p Point, c Cell) bool {
	if !d.InBounds(p) {
		return false
	}
	d.Cells[p.Y*d.Width+p.X] = c
	return true
}

func (d *Document) Validate() error {
	if d == nil {
		return fmt.Errorf("document is nil")
	}
	if d.Version != CurrentVersion {
		return fmt.Errorf("unsupported document version %d (expected %d)", d.Version, CurrentVersion)
	}
	if err := validateDimensions(d.Width, d.Height, d.Aspect); err != nil {
		return err
	}
	if len(d.Cells) != d.Width*d.Height {
		return fmt.Errorf("canvas has %d cells; dimensions require %d", len(d.Cells), d.Width*d.Height)
	}
	for i, c := range d.Cells {
		if err := c.validate(); err != nil {
			return fmt.Errorf("cell (%d,%d): %w", i%d.Width, i/d.Width, err)
		}
	}
	return nil
}

type PaintMode int

const (
	PaintGlyph PaintMode = iota
	PaintAuto
	PaintTransparent
	PaintDelete
)

// Paint describes the character and colors to use for a drawing operation.
type Paint struct {
	Mode  PaintMode
	Glyph rune
	FG    Color
	BG    Color
}

// Cell returns the replacement cell; false means leave the old cell alone.
func (p Paint) Cell(auto rune) (Cell, bool) {
	var r rune
	switch p.Mode {
	case PaintGlyph:
		r = p.Glyph
	case PaintAuto:
		r = auto
	case PaintDelete:
		return Blank(), true
	case PaintTransparent:
		return Cell{}, false
	default:
		return Cell{}, false
	}
	c := Cell{Rune: r, FG: p.FG, BG: p.BG}
	if c.validate() != nil {
		return Cell{}, false
	}
	return c, true
}
