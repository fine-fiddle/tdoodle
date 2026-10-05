package geometry

import (
	"math"
	"testing"

	"tdoodle/internal/canvas"
)

func TestPerpendicularCompensatesForCellAspect(t *testing.T) {
	u := Vector{X: 6, Y: 2}
	for _, aspect := range []float64{1, 2, .75} {
		v := Perpendicular(u, aspect)
		// Measure both vectors in physical coordinates (cell width = 1).
		if dot := u.X*v.X + u.Y*aspect*v.Y*aspect; math.Abs(dot) > 1e-12 {
			t.Fatalf("aspect %g: physical radii are not perpendicular: %g", aspect, dot)
		}
		if delta := math.Hypot(u.X, u.Y*aspect) - math.Hypot(v.X, v.Y*aspect); math.Abs(delta) > 1e-12 {
			t.Fatalf("aspect %g: physical radii differ in length: %g", aspect, delta)
		}
	}
}

func TestLineEndpointsConnectedAndGlyphs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		a, b   canvas.Point
		aspect float64
		want   rune
	}{
		{"horizontal", canvas.Point{X: 1, Y: 2}, canvas.Point{X: 15, Y: 2}, 2, '-'},
		{"vertical", canvas.Point{X: 2, Y: 1}, canvas.Point{X: 2, Y: 12}, 2, '|'},
		{"falling", canvas.Point{X: 1, Y: 1}, canvas.Point{X: 15, Y: 6}, 2, '\\'},
		{"rising", canvas.Point{X: 1, Y: 6}, canvas.Point{X: 15, Y: 1}, 2, '/'},
		{"reversed", canvas.Point{X: 15, Y: 6}, canvas.Point{X: 1, Y: 1}, 2, '\\'},
		{"square cells", canvas.Point{X: 1, Y: 1}, canvas.Point{X: 6, Y: 3}, 1, '-'},
		{"terminal cells", canvas.Point{X: 1, Y: 1}, canvas.Point{X: 6, Y: 3}, 2, '\\'},
		{"invalid aspect", canvas.Point{X: 1, Y: 1}, canvas.Point{X: 6, Y: 3}, 0, '\\'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Line(tc.a, tc.b, tc.aspect, 20, 15)
			for p, ch := range s.Outline {
				if ch != tc.want {
					t.Errorf("cell %v has glyph %q, want %q", p, ch, tc.want)
				}
			}
			if _, ok := s.Outline[tc.a]; !ok {
				t.Errorf("missing first endpoint %v", tc.a)
			}
			if _, ok := s.Outline[tc.b]; !ok {
				t.Errorf("missing final endpoint %v", tc.b)
			}
			assertConnected(t, s.Outline)
			assertClipped(t, s, 20, 15)
		})
	}
}

func TestLineClipsBeforeTraversing(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	s := Line(canvas.Point{X: -maxInt, Y: 4}, canvas.Point{X: maxInt, Y: 4}, 2, 20, 10)
	if len(s.Outline) != 20 {
		t.Fatalf("long clipped horizontal line has %d cells, want 20", len(s.Outline))
	}
	for x := 0; x < 20; x++ {
		if s.Outline[canvas.Point{X: x, Y: 4}] != '-' {
			t.Errorf("missing cell %d of clipped line", x)
		}
	}
	if s := Line(canvas.Point{X: -100, Y: -100}, canvas.Point{X: -1, Y: -1}, 2, 20, 10); len(s.Outline) != 0 {
		t.Fatalf("fully offscreen line has %d cells", len(s.Outline))
	}
}

func TestRectangleReversedCornersAndFill(t *testing.T) {
	s := Rectangle(canvas.Point{X: 7, Y: 6}, canvas.Point{X: 2, Y: 1}, 12, 10)
	if len(s.Outline) != 20 || len(s.Interior) != 16 {
		t.Fatalf("rectangle has %d outline and %d interior cells, want 20 and 16", len(s.Outline), len(s.Interior))
	}
	for _, p := range []canvas.Point{{X: 2, Y: 1}, {X: 2, Y: 6}, {X: 7, Y: 1}, {X: 7, Y: 6}} {
		if s.Outline[p] != '+' {
			t.Errorf("corner %v = %q, want +", p, s.Outline[p])
		}
	}
	for _, p := range s.Interior {
		if p.X <= 2 || p.X >= 7 || p.Y <= 1 || p.Y >= 6 {
			t.Errorf("invalid interior point %v", p)
		}
	}
	assertConnected(t, s.Outline)
	assertClipped(t, s, 12, 10)
}

func TestRectangleClippingDoesNotInventEdges(t *testing.T) {
	s := Rectangle(canvas.Point{X: -3, Y: -3}, canvas.Point{X: 4, Y: 4}, 8, 8)
	if _, ok := s.Outline[canvas.Point{}]; ok {
		t.Fatal("clipping moved an invisible rectangle corner to the screen origin")
	}
	if !hasInterior(s, canvas.Point{}) || hasInterior(s, canvas.Point{X: 5, Y: 1}) {
		t.Fatal("clipped rectangle interior does not respect original edges")
	}
	assertClipped(t, s, 8, 8)
	maxInt := int(^uint(0) >> 1)
	whole := Rectangle(canvas.Point{X: -maxInt - 1, Y: -maxInt - 1}, canvas.Point{X: maxInt, Y: maxInt}, 8, 8)
	if len(whole.Outline) != 0 || len(whole.Interior) != 64 {
		t.Fatalf("rectangle surrounding canvas has %d outline and %d interior cells", len(whole.Outline), len(whole.Interior))
	}
}

func TestEllipseClosureAndAffineFill(t *testing.T) {
	c := canvas.Point{X: 20, Y: 15}
	for _, radii := range [][2]Vector{
		{{X: 12}, {Y: 6}},
		{{X: 9, Y: 4}, {X: -7, Y: 6}},
		{{X: 11, Y: 3}, {X: 7, Y: 8}},
	} {
		u, v := radii[0], radii[1]
		s := Ellipse(c, u, v, 2, 50, 40)
		assertConnected(t, s.Outline)
		assertClipped(t, s, 50, 40)
		if !hasInterior(s, c) {
			t.Fatalf("ellipse with %v and %v does not enclose its center", u, v)
		}
		det := u.X*v.Y - u.Y*v.X
		for y := 0; y < 40; y++ {
			for x := 0; x < 50; x++ {
				p := canvas.Point{X: x, Y: y}
				if _, border := s.Outline[p]; border {
					continue
				}
				dx, dy := float64(x-c.X), float64(y-c.Y)
				a, b := (dx*v.Y-dy*v.X)/det, (u.X*dy-u.Y*dx)/det
				want := a*a+b*b < 1-1e-12
				if hasInterior(s, p) != want {
					t.Errorf("ellipse %v/%v fill at %v is %t, want %t", u, v, p, !want, want)
				}
			}
		}
		// A closed digital boundary separates the center from the viewport edge
		// for a four-neighbor flood, even though filling itself is mathematical.
		if canReachEdge(c, s.Outline, 50, 40) {
			t.Fatalf("ellipse with %v and %v has a gap in its perimeter", u, v)
		}
	}
}

func TestEllipseClippedFillUsesMathematicalInterior(t *testing.T) {
	s := Ellipse(canvas.Point{X: -5, Y: 10}, Vector{X: 15}, Vector{Y: 8}, 2, 20, 21)
	if !hasInterior(s, canvas.Point{X: 0, Y: 10}) {
		t.Fatal("open clipped ellipse lost its visible mathematical interior")
	}
	for _, p := range []canvas.Point{{X: 0, Y: 0}, {X: 15, Y: 10}, {X: 0, Y: 20}} {
		if hasInterior(s, p) {
			t.Errorf("clipped ellipse incorrectly fills outside point %v", p)
		}
	}
	assertClipped(t, s, 20, 21)
	// No perimeter is visible, but the whole screen lies inside this ellipse.
	whole := Ellipse(canvas.Point{X: 10, Y: 10}, Vector{X: 1e12}, Vector{Y: 1e12}, 2, 20, 21)
	if len(whole.Outline) != 0 || len(whole.Interior) != 20*21 {
		t.Fatalf("huge ellipse has %d outline and %d interior cells, want 0 and 420", len(whole.Outline), len(whole.Interior))
	}
}

func TestEllipseDegeneracyAndTinyRadii(t *testing.T) {
	c := canvas.Point{X: 10, Y: 10}
	point := Ellipse(c, Vector{}, Vector{}, 2, 30, 20)
	if len(point.Outline) != 1 || len(point.Interior) != 0 {
		t.Fatalf("zero-radius ellipse = %v", point)
	}
	line := Ellipse(c, Vector{X: 3}, Vector{X: 4}, 2, 30, 20)
	if len(line.Outline) != 11 || len(line.Interior) != 0 {
		t.Fatalf("collinear radii should yield radius hypot(3,4)=5, got %d cells", len(line.Outline))
	}
	for _, tc := range []struct{ u, v Vector }{
		{Vector{X: .2}, Vector{Y: .2}},
		{Vector{X: 3, Y: 2}, Vector{X: 6, Y: 4}},
		{Vector{}, Vector{Y: 5}},
		{Vector{X: 1e15}, Vector{Y: 2}},
	} {
		s := Ellipse(c, tc.u, tc.v, 2, 30, 20)
		if len(s.Outline) == 0 && len(s.Interior) == 0 {
			t.Errorf("ellipse %v/%v disappeared", tc.u, tc.v)
		}
		assertClipped(t, s, 30, 20)
	}
	if s := Ellipse(c, Vector{X: math.Inf(1)}, Vector{Y: 2}, 2, 30, 20); len(s.Outline) != 0 || len(s.Interior) != 0 {
		t.Fatal("nonfinite radius should yield an empty shape")
	}
}

func TestShapesOnEmptyCanvas(t *testing.T) {
	for _, s := range []Shape{
		Line(canvas.Point{}, canvas.Point{X: 1}, 2, 0, 10),
		Rectangle(canvas.Point{}, canvas.Point{X: 1, Y: 1}, 10, 0),
		Ellipse(canvas.Point{}, Vector{X: 1}, Vector{Y: 1}, 2, -1, 10),
	} {
		if len(s.Outline)+len(s.Interior) != 0 {
			t.Fatal("shape exists on an empty canvas")
		}
	}
}

func assertConnected(t *testing.T, outline map[canvas.Point]rune) {
	t.Helper()
	if len(outline) == 0 {
		t.Fatal("expected a connected nonempty outline")
	}
	var start canvas.Point
	for p := range outline {
		start = p
		break
	}
	seen := map[canvas.Point]bool{start: true}
	queue := []canvas.Point{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := canvas.Point{X: p.X + dx, Y: p.Y + dy}
				if _, ok := outline[n]; ok && !seen[n] {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
	}
	if len(seen) != len(outline) {
		t.Fatalf("outline is disconnected: reached %d of %d cells", len(seen), len(outline))
	}
}

func assertClipped(t *testing.T, s Shape, w, h int) {
	t.Helper()
	for p := range s.Outline {
		if !visible(p, w, h) {
			t.Errorf("offscreen outline cell %v", p)
		}
	}
	seen := make(map[canvas.Point]bool)
	for _, p := range s.Interior {
		if !visible(p, w, h) {
			t.Errorf("offscreen interior cell %v", p)
		}
		if _, border := s.Outline[p]; border {
			t.Errorf("interior overlaps outline at %v", p)
		}
		if seen[p] {
			t.Errorf("duplicate interior cell %v", p)
		}
		seen[p] = true
	}
}

func hasInterior(s Shape, p canvas.Point) bool {
	for _, inside := range s.Interior {
		if inside == p {
			return true
		}
	}
	return false
}

func canReachEdge(start canvas.Point, outline map[canvas.Point]rune, w, h int) bool {
	seen := map[canvas.Point]bool{start: true}
	queue := []canvas.Point{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p.X == 0 || p.X == w-1 || p.Y == 0 || p.Y == h-1 {
			return true
		}
		for _, delta := range []canvas.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			n := canvas.Point{X: p.X + delta.X, Y: p.Y + delta.Y}
			if _, border := outline[n]; !border && !seen[n] && visible(n, w, h) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return false
}
