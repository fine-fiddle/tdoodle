// Package geometry rasterizes drawing tools without changing the document.
// Coordinates describe cell centers; aspect is the cell height divided by its width.
package geometry

import (
	"math"
	"sort"

	"tdoodle/internal/canvas"
)

// Vector is a radius handle in terminal-cell coordinates.
type Vector struct {
	X, Y float64
}

// Shape separates the perimeter from its mathematically enclosed cells. An
// interior point never also appears in Outline. Neither collection includes
// points outside the supplied canvas dimensions.
type Shape struct {
	Outline  map[canvas.Point]rune
	Interior []canvas.Point
}

func emptyShape() Shape { return Shape{Outline: make(map[canvas.Point]rune)} }

// Perpendicular returns an equally long, physically perpendicular radius.
// A terminal cell commonly has twice as much height as width (aspect = 2).
func Perpendicular(u Vector, aspect float64) Vector {
	aspect = validAspect(aspect)
	return Vector{X: -u.Y * aspect, Y: u.X / aspect}
}

// Line draws a connected Bresenham line, choosing an ASCII character from its
// physical slope. Clipping happens before traversal, so distant endpoints do
// not cause work proportional to their distance from the canvas.
func Line(a, b canvas.Point, aspect float64, w, h int) Shape {
	s := emptyShape()
	if w <= 0 || h <= 0 {
		return s
	}
	ax, ay, bx, by := float64(a.X), float64(a.Y), float64(b.X), float64(b.Y)
	drawSegment(s.Outline, ax, ay, bx, by, glyph(bx-ax, by-ay, aspect), w, h)
	return s
}

// Rectangle draws an axis-aligned rectangle. Handles may be set in any order.
// Its interior is defined by the original edges even when some are offscreen.
func Rectangle(a, b canvas.Point, w, h int) Shape {
	s := emptyShape()
	if w <= 0 || h <= 0 {
		return s
	}
	x0, x1 := min(a.X, b.X), max(a.X, b.X)
	y0, y1 := min(a.Y, b.Y), max(a.Y, b.Y)
	if x0 == x1 || y0 == y1 {
		ch := '-'
		if x0 == x1 && y0 != y1 {
			ch = '|'
		}
		drawSegment(s.Outline, float64(x0), float64(y0), float64(x1), float64(y1), ch, w, h)
		return s
	}
	drawSegment(s.Outline, float64(x0), float64(y0), float64(x1), float64(y0), '-', w, h)
	drawSegment(s.Outline, float64(x0), float64(y1), float64(x1), float64(y1), '-', w, h)
	drawSegment(s.Outline, float64(x0), float64(y0), float64(x0), float64(y1), '|', w, h)
	drawSegment(s.Outline, float64(x1), float64(y0), float64(x1), float64(y1), '|', w, h)
	for _, p := range []canvas.Point{{X: x0, Y: y0}, {X: x0, Y: y1}, {X: x1, Y: y0}, {X: x1, Y: y1}} {
		if visible(p, w, h) {
			s.Outline[p] = '+'
		}
	}
	// Iterate visible cells instead of incrementing an offscreen bound: this
	// also avoids overflowing when a handle is the largest possible integer.
	for y := max(0, y0); y < h && y < y1; y++ {
		if y <= y0 {
			continue
		}
		for x := max(0, x0); x < w && x < x1; x++ {
			if x > x0 {
				s.Interior = append(s.Interior, canvas.Point{X: x, Y: y})
			}
		}
	}
	return s
}

// Ellipse uses C + u*cos(t) + v*sin(t). The second handle can point in any
// direction: it need not be perpendicular to the first. Collinear handles
// collapse to a line; zero handles collapse to a point.
//
// The outline is sampled only where the curve can reach the visible canvas,
// then joined with Bresenham segments. Interior cells are found by the inverse
// affine transform, never by flooding the clipped perimeter.
func Ellipse(center canvas.Point, u, v Vector, aspect float64, w, h int) Shape {
	s := emptyShape()
	if w <= 0 || h <= 0 || !finite(u.X, u.Y, v.X, v.Y) {
		return s
	}
	cx, cy := float64(center.X), float64(center.Y)
	scale := max(math.Abs(u.X), math.Abs(u.Y), math.Abs(v.X), math.Abs(v.Y))
	if scale == 0 {
		if visible(center, w, h) {
			s.Outline[center] = '-'
		}
		return s
	}
	// Normalization keeps determinant tests finite for large handles.
	un, vn := Vector{u.X / scale, u.Y / scale}, Vector{v.X / scale, v.Y / scale}
	det := un.X*vn.Y - un.Y*vn.X
	if det == 0 {
		base := u
		if math.Hypot(v.X, v.Y) > math.Hypot(u.X, u.Y) {
			base = v
		}
		length := math.Hypot(base.X, base.Y)
		dx, dy := base.X/length, base.Y/length
		radius := math.Hypot(u.X*dx+u.Y*dy, v.X*dx+v.Y*dy)
		drawSegment(s.Outline, cx-dx*radius, cy-dy*radius, cx+dx*radius, cy+dy*radius, glyph(dx, dy, aspect), w, h)
		return s
	}

	const turn = 2 * math.Pi
	angles := []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, turn}
	// Extrema partition the curve into intervals monotone in both coordinates.
	// Viewport intersections discard invisible intervals, including a huge
	// ellipse with only a very short arc on the screen.
	addEvents := func(c, a, b, low, high float64) {
		r := math.Hypot(a, b)
		if r == 0 {
			return
		}
		phase := math.Atan2(b, a)
		angles = append(angles, normalizeAngle(phase), normalizeAngle(phase+math.Pi))
		for _, edge := range []float64{low, high} {
			q := (edge - c) / r
			if q >= -1 && q <= 1 {
				d := math.Acos(q)
				angles = append(angles, normalizeAngle(phase-d), normalizeAngle(phase+d))
			}
		}
	}
	addEvents(cx, u.X, v.X, -1, float64(w))
	addEvents(cy, u.Y, v.Y, -1, float64(h))
	sort.Float64s(angles)
	point := func(t float64) (float64, float64) {
		st, ct := math.Sincos(t)
		return cx + u.X*ct + v.X*st, cy + u.Y*ct + v.Y*st
	}
	// Each monotone visible interval has at most w+h cell transitions. This
	// budget and the recursion limit make even pathological inputs bounded.
	budget := sampleBudget(w, h)
	var sample func(float64, float64, float64, float64, float64, float64, int)
	sample = func(t0, t1, x0, y0, x1, y1 float64, depth int) {
		if budget <= 0 {
			return
		}
		if depth < 48 && max(math.Abs(x1-x0), math.Abs(y1-y0)) > .5 {
			tm := (t0 + t1) / 2
			xm, ym := point(tm)
			if tm != t0 && tm != t1 && finite(xm, ym) {
				sample(t0, tm, x0, y0, xm, ym, depth+1)
				sample(tm, t1, xm, ym, x1, y1, depth+1)
				return
			}
		}
		budget--
		st, ct := math.Sincos((t0 + t1) / 2)
		drawSegment(s.Outline, x0, y0, x1, y1, glyph(-u.X*st+v.X*ct, -u.Y*st+v.Y*ct, aspect), w, h)
	}
	for i := 1; i < len(angles) && budget > 0; i++ {
		t0, t1 := angles[i-1], angles[i]
		if t1-t0 < 1e-14 {
			continue
		}
		xm, ym := point((t0 + t1) / 2)
		if xm < -1 || xm > float64(w) || ym < -1 || ym > float64(h) {
			continue
		}
		x0, y0 := point(t0)
		x1, y1 := point(t1)
		sample(t0, t1, x0, y0, x1, y1, 0)
	}

	ex, ey := math.Hypot(u.X, v.X), math.Hypot(u.Y, v.Y)
	x0, x1 := scanBounds(cx-ex, cx+ex, w)
	y0, y1 := scanBounds(cy-ey, cy+ey, h)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			p := canvas.Point{X: x, Y: y}
			if _, perimeter := s.Outline[p]; perimeter {
				continue
			}
			dx, dy := (float64(x)-cx)/scale, (float64(y)-cy)/scale
			a := (dx*vn.Y - dy*vn.X) / det
			b := (un.X*dy - un.Y*dx) / det
			if a*a+b*b < 1-1e-12 {
				s.Interior = append(s.Interior, p)
			}
		}
	}
	return s
}

func validAspect(aspect float64) float64 {
	if aspect <= 0 || math.IsNaN(aspect) || math.IsInf(aspect, 0) {
		return 2
	}
	return aspect
}

func glyph(dx, dy, aspect float64) rune {
	dy *= validAspect(aspect)
	ax, ay := math.Abs(dx), math.Abs(dy)
	const diagonalThreshold = .41421356237309503 // tan(pi/8)
	if ay <= ax*diagonalThreshold {
		return '-'
	}
	if ax <= ay*diagonalThreshold {
		return '|'
	}
	if (dx >= 0) == (dy >= 0) {
		return '\\'
	}
	return '/'
}

func visible(p canvas.Point, w, h int) bool {
	return p.X >= 0 && p.X < w && p.Y >= 0 && p.Y < h
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func normalizeAngle(t float64) float64 {
	t = math.Mod(t, 2*math.Pi)
	if t < 0 {
		t += 2 * math.Pi
	}
	return t
}

func sampleBudget(w, h int) int {
	// Valid document dimensions are much smaller than this cap. Avoid integer
	// overflow here even if a caller supplies invalid canvas dimensions.
	if w > 1<<18 || h > 1<<18 {
		return 1 << 20
	}
	return min(32*(w+h)+128, 1<<20)
}

func scanBounds(low, high float64, size int) (int, int) {
	if high < 0 || low > float64(size-1) {
		return 0, 0
	}
	start, end := 0, size
	if low > 0 {
		start = int(math.Ceil(low))
	}
	if high < float64(size-1) {
		end = int(math.Floor(high)) + 1
	}
	return start, end
}

// clipSegment uses Cohen-Sutherland clipping. Assigning the intersection's
// clipped coordinate exactly avoids losing tiny visible segments when handles
// are very distant from the viewport.
func clipSegment(ax, ay, bx, by float64, w, h int) (float64, float64, float64, float64, bool) {
	if !finite(ax, ay, bx, by) {
		return 0, 0, 0, 0, false
	}
	left, top := -.5, -.5
	right, bottom := float64(w)-.5, float64(h)-.5
	code := func(x, y float64) uint8 {
		var c uint8
		if x < left {
			c |= 1
		}
		if x > right {
			c |= 2
		}
		if y < top {
			c |= 4
		}
		if y > bottom {
			c |= 8
		}
		return c
	}
	for attempts := 0; attempts < 16; attempts++ {
		ca, cb := code(ax, ay), code(bx, by)
		if ca|cb == 0 {
			return ax, ay, bx, by, true
		}
		if ca&cb != 0 {
			break
		}
		outside := ca
		if outside == 0 {
			outside = cb
		}
		var x, y float64
		switch {
		case outside&4 != 0:
			y = top
			x = ax + (bx-ax)*((top-ay)/(by-ay))
		case outside&8 != 0:
			y = bottom
			x = ax + (bx-ax)*((bottom-ay)/(by-ay))
		case outside&2 != 0:
			x = right
			y = ay + (by-ay)*((right-ax)/(bx-ax))
		default:
			x = left
			y = ay + (by-ay)*((left-ax)/(bx-ax))
		}
		if !finite(x, y) {
			break
		}
		if outside == ca {
			ax, ay = x, y
		} else {
			bx, by = x, y
		}
	}
	return 0, 0, 0, 0, false
}

func drawSegment(outline map[canvas.Point]rune, ax, ay, bx, by float64, ch rune, w, h int) {
	ax, ay, bx, by, ok := clipSegment(ax, ay, bx, by, w, h)
	if !ok {
		return
	}
	round := func(value float64, size int) int {
		return max(0, min(size-1, int(math.Floor(value+.5))))
	}
	x0, y0, x1, y1 := round(ax, w), round(ay, h), round(bx, w), round(by, h)
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		outline[canvas.Point{X: x0, Y: y0}] = ch
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
