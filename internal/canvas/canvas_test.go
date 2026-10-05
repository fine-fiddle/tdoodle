package canvas

import (
	"math"
	"testing"
)

func mustDocument(t *testing.T, w, h int) *Document {
	t.Helper()
	d, err := New(w, h, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDocumentCoordinates(t *testing.T) {
	d := mustDocument(t, 3, 2)
	for _, c := range d.Cells {
		if c != Blank() {
			t.Fatalf("new document cell = %#v, want blank", c)
		}
	}
	c := Cell{Rune: 'X', FG: Orange, BG: Blue}
	if !d.Set(Point{2, 1}, c) || d.At(Point{2, 1}) != c {
		t.Fatal("last cell was not set")
	}
	for _, p := range []Point{{-1, 0}, {0, -1}, {3, 0}, {0, 2}, {math.MaxInt, math.MaxInt}} {
		if d.InBounds(p) || d.Set(p, c) || d.At(p) != Blank() {
			t.Errorf("out-of-bounds point %#v was accessible", p)
		}
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRejectsInvalidSizeAndAspect(t *testing.T) {
	for _, tc := range []struct {
		name   string
		w, h   int
		aspect float64
	}{
		{"zero width", 0, 10, 0.5},
		{"negative height", 10, -1, 0.5},
		{"huge width", math.MaxInt, 10, 0.5},
		{"huge height", 10, math.MaxInt, 0.5},
		{"too many cells", MaxWidth, MaxHeight, 0.5},
		{"zero aspect", 10, 10, 0},
		{"negative aspect", 10, 10, -1},
		{"large aspect", 10, 10, MaxAspect + 1},
		{"nan aspect", 10, 10, math.NaN()},
		{"infinite aspect", 10, 10, math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.w, tc.h, tc.aspect); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
}

func TestValidateRejectsInvalidDocument(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Document)
	}{
		{"future version", func(d *Document) { d.Version++ }},
		{"missing cells", func(d *Document) { d.Cells = nil }},
		{"extra cells", func(d *Document) { d.Cells = append(d.Cells, Blank()) }},
		{"control", func(d *Document) { d.Cells[0].Rune = '\n' }},
		{"delete", func(d *Document) { d.Cells[0].Rune = 127 }},
		{"unicode", func(d *Document) { d.Cells[0].Rune = 'é' }},
		{"negative rune", func(d *Document) { d.Cells[0].Rune = -1 }},
		{"unknown foreground", func(d *Document) { d.Cells[0].FG = "chartreuse" }},
		{"unknown background", func(d *Document) { d.Cells[0].BG = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := mustDocument(t, 1, 1)
			tc.mutate(d)
			if err := d.Validate(); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	var nilDocument *Document
	if nilDocument.Validate() == nil {
		t.Fatal("nil document accepted")
	}
}

func TestPaintSemantics(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode PaintMode
		want Cell
		ok   bool
	}{
		{"glyph", PaintGlyph, Cell{Rune: '*', FG: Red, BG: Green}, true},
		{"auto", PaintAuto, Cell{Rune: '-', FG: Red, BG: Green}, true},
		{"transparent", PaintTransparent, Cell{}, false},
		{"delete", PaintDelete, Blank(), true},
		{"unknown", PaintMode(100), Cell{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Paint{Mode: tc.mode, Glyph: '*', FG: Red, BG: Green}
			got, ok := p.Cell('-')
			if got != tc.want || ok != tc.ok {
				t.Fatalf("Cell = %#v, %v; want %#v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
	for _, p := range []Paint{
		{Mode: PaintGlyph, Glyph: '\t', FG: White, BG: Black},
		{Mode: PaintGlyph, Glyph: 'x', FG: "", BG: Black},
		{Mode: PaintAuto, FG: White, BG: Black},
	} {
		if _, ok := p.Cell('é'); ok {
			t.Errorf("invalid paint %#v accepted", p)
		}
	}
}

func TestPalette(t *testing.T) {
	if len(Colors) != 10 || Colors[0] != Red || Colors[9] != Black {
		t.Fatal("incorrect numbered palette order")
	}
	seen := make(map[Color]bool)
	for _, c := range Colors {
		if !c.Valid() || seen[c] {
			t.Fatalf("invalid or duplicated palette color %q", c)
		}
		seen[c] = true
	}
}
