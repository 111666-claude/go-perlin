package perlin

import (
	"math"
	"math/rand"
	"testing"
)

const (
	seed = 123
)

// closeEnough compares noise values allowing only floating-point rounding
// drift introduced by wrapping non-integer periods.
func closeEnough(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func Test_PerlinNoise1D(t *testing.T) {
	expected := 0.0
	p := NewPerlin(2, 2, 3, seed)
	noise := p.Noise1D(10)
	if noise != expected {
		t.Fail()
		t.Logf("Wrong node result: given: %f, expected: %f", noise, expected)
	}
}

func Test_PerlinNoise2D(t *testing.T) {
	expected := 0.0
	p := NewPerlin(2, 2, 3, seed)
	noise := p.Noise2D(10, 10)
	if noise != expected {
		t.Fail()
		t.Logf("Wrong node result: given: %f, expected: %f", noise, expected)
	}
}

func Test_PerlinNoise3D(t *testing.T) {
	expected := 0.0
	p := NewPerlin(2, 2, 3, seed)
	noise := p.Noise3D(10, 10, 10)
	if noise != expected {
		t.Fail()
		t.Logf("Wrong node result: given: %f, expected: %f", noise, expected)
	}
}

// frac returns the fractional part of x in [0,1).
func frac(x float64) float64 {
	return x - math.Floor(x)
}

// Test_PerlinNoise2DPeriodic_TileEquality checks the core contract for the
// integer periods width=6, height=4: shifting a query point by a whole
// period on either or both axes must return the same value, bit for bit,
// including at the four corners and for negative and non-integer
// coordinates.
func Test_PerlinNoise2DPeriodic_TileEquality(t *testing.T) {
	const (
		width  = 6.0
		height = 4.0
	)
	cases := []struct {
		name string
		x, y float64
	}{
		{"origin", 0, 0},
		{"fractional", 1.25, 2.75},
		{"near edges", width - 0.1, height - 0.1},
		{"negative", -3.5, -1.25},
		{"negative fractional", -0.3, -3.7},
		{"far outside", 47.8, -29.4},
		{"on period boundary", width, height},
		{"half period", width / 2, height / 2},
	}
	type cfg struct {
		name       string
		alpha      float64
		beta       float64
		n          int32
		useRandSrc bool
	}
	configs := []cfg{
		{"default", 2, 2, 3, false},
		{"high octaves", 2, 2, 8, false},
		{"non integer beta", 2, 2.5, 4, false},
		{"single octave", 2, 2, 1, false},
		{"rand source", 2, 2, 3, true},
	}
	for _, c := range configs {
		var p *Perlin
		if c.useRandSrc {
			p = NewPerlinRandSource(c.alpha, c.beta, c.n, rand.NewSource(seed))
		} else {
			p = NewPerlin(c.alpha, c.beta, c.n, seed)
		}
		for _, tc := range cases {
			base := p.Noise2DPeriodic(width, height, tc.x, tc.y)
			shiftX := p.Noise2DPeriodic(width, height, tc.x+width, tc.y)
			shiftY := p.Noise2DPeriodic(width, height, tc.x, tc.y+height)
			shiftXY := p.Noise2DPeriodic(width, height, tc.x+width, tc.y+height)
			// Integer periods and pre-wrapping give bit-identical results.
			if base != shiftX || base != shiftY || base != shiftXY {
				t.Errorf("%s/%s: periodic shift changed the value: base=%v +x=%v +y=%v +xy=%v",
					c.name, tc.name, base, shiftX, shiftY, shiftXY)
			}
			if math.IsNaN(base) || math.IsInf(base, 0) {
				t.Errorf("%s/%s: expected finite result, got %v", c.name, tc.name, base)
			}
		}
	}
}

// Test_PerlinNoise2DPeriodic_NonIntegerPeriod verifies repetition with
// non-integer periods and periods smaller than one lattice cell. Equality
// is checked against floating-point rounding tolerance: with arbitrary
// non-integer periods the two queries take different rounding paths, so
// bit-identical results are not guaranteed by IEEE arithmetic even though
// the underlying function is mathematically exactly periodic.
func Test_PerlinNoise2DPeriodic_NonIntegerPeriod(t *testing.T) {
	periods := []struct {
		width, height float64
		tol           float64
	}{
		{6.5, 4.25, 1e-12},
		{0.7, 0.3, 1e-12}, // whole base octave collapses onto a single cell
		{1.0, 1.0, 0},     // integer period stays bit exact
		{100.7, 0.9, 1e-12},
		{3.3333333333, 7.6666666667, 1e-12},
	}
	p := NewPerlin(2, 2, 5, seed)
	for _, pr := range periods {
		for i := 0; i < 20; i++ {
			x := frac(math.Sin(float64(i)*12.9898)*43758.5453) * pr.width
			y := frac(math.Sin(float64(i)*78.233)*12543.1234) * pr.height
			base := p.Noise2DPeriodic(pr.width, pr.height, x, y)
			if got := p.Noise2DPeriodic(pr.width, pr.height, x+pr.width, y); !closeEnough(got, base, pr.tol) {
				t.Errorf("width=%v height=%v: x shift mismatch at (%v,%v): %v != %v",
					pr.width, pr.height, x, y, got, base)
			}
			if got := p.Noise2DPeriodic(pr.width, pr.height, x, y+pr.height); !closeEnough(got, base, pr.tol) {
				t.Errorf("width=%v height=%v: y shift mismatch at (%v,%v): %v != %v",
					pr.width, pr.height, x, y, got, base)
			}
			if got := p.Noise2DPeriodic(pr.width, pr.height, x-pr.width, y-pr.height); !closeEnough(got, base, pr.tol) {
				t.Errorf("width=%v height=%v: negative shift mismatch: %v != %v",
					pr.width, pr.height, got, base)
			}
		}
	}
}

// Test_PerlinNoise2DPeriodic_EdgeContinuity verifies that the field meets
// smoothly across every seam. Values sampled on the two sides of an edge
// agree, and the local variation (the one-sided first derivative) crossing
// a seam matches from inside the tile and from the next tile. Both
// axis-only and dual-axis tiling are covered this way.
//
// Each slope uses the same step and the same finite-difference formula on
// both sides, so the truncation error cancels in the comparison and only
// the mathematical C1 continuity of the field is exercised.
func Test_PerlinNoise2DPeriodic_EdgeContinuity(t *testing.T) {
	const eps = 1e-9
	seams := []struct {
		name          string
		width, height float64
	}{
		{"integer periods", 6, 4},
		{"non integer periods", 6.5, 4.25},
		{"tiny periods", 0.7, 0.3},
		{"x only tiling, large y", 6, 100},
		{"y only tiling, large x", 100, 4},
	}
	for _, s := range seams {
		p := NewPerlin(2, 2, 4, seed)
		checks := []struct {
			name   string
			xa, ya float64
			xb, yb float64
		}{
			// vertical seam: left of x=width vs right of x=0
			{"x seam", s.width - eps, 1.7, -eps, 1.7},
			// horizontal seam: below y=height vs above y=0
			{"y seam", 2.3, s.height - eps, 2.3, -eps},
			// corner where both seams meet
			{"corner", s.width - eps, s.height - eps, -eps, -eps},
		}
		for _, c := range checks {
			a := p.Noise2DPeriodic(s.width, s.height, c.xa, c.ya)
			b := p.Noise2DPeriodic(s.width, s.height, c.xb, c.yb)
			if math.Abs(a-b) > 1e-11 {
				t.Errorf("%s/%s: value discontinuity: %v != %v (diff %v)",
					s.name, c.name, a, b, math.Abs(a-b))
			}
		}

		// Local variation across the x seam: the derivative at the seam
		// estimated from two points inside the tile must equal the
		// estimate from two points in the next (periodically identical)
		// tile. Both differences span the same width 2*eps.
		dxInside := (p.Noise2DPeriodic(s.width, s.height, s.width-eps, 1.7) -
			p.Noise2DPeriodic(s.width, s.height, s.width-3*eps, 1.7)) / (2 * eps)
		dxOutside := (p.Noise2DPeriodic(s.width, s.height, eps, 1.7) -
			p.Noise2DPeriodic(s.width, s.height, -eps, 1.7)) / (2 * eps)
		// 1e-3 is well above the O(eps) truncation and round-off floor
		// of these tiny finite differences (~1e-4) and orders of
		// magnitude below a genuine derivative jump, which would be O(1).
		if math.Abs(dxInside-dxOutside) > 1e-3 {
			t.Errorf("%s: x seam slopes differ: %v != %v",
				s.name, dxInside, dxOutside)
		}

		dyInside := (p.Noise2DPeriodic(s.width, s.height, 2.3, s.height-eps) -
			p.Noise2DPeriodic(s.width, s.height, 2.3, s.height-3*eps)) / (2 * eps)
		dyOutside := (p.Noise2DPeriodic(s.width, s.height, 2.3, eps) -
			p.Noise2DPeriodic(s.width, s.height, 2.3, -eps)) / (2 * eps)
		if math.Abs(dyInside-dyOutside) > 1e-3 {
			t.Errorf("%s: y seam slopes differ: %v != %v",
				s.name, dyInside, dyOutside)
		}
	}
}

// Test_PerlinNoise2DPeriodic_Corners checks that all four corners of a tile
// collapse onto a single value (the wrapping origin).
func Test_PerlinNoise2DPeriodic_Corners(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	const w, h = 6.0, 4.0
	c00 := p.Noise2DPeriodic(w, h, 0, 0)
	corner := []float64{
		p.Noise2DPeriodic(w, h, w, 0),
		p.Noise2DPeriodic(w, h, 0, h),
		p.Noise2DPeriodic(w, h, w, h),
		p.Noise2DPeriodic(w, h, -w, -h),
	}
	for i, v := range corner {
		if v != c00 {
			t.Errorf("corner %d: %v != origin %v", i, v, c00)
		}
	}
}

// Test_PerlinNoise2DPeriodic_NegativeCoordinates checks deterministic
// wrapping of negative coordinates onto the base tile.
func Test_PerlinNoise2DPeriodic_NegativeCoordinates(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	const w, h = 6.0, 4.0
	for i := 0; i < 50; i++ {
		x := -float64(i) * 0.37
		y := -float64(i) * 0.61
		neg := p.Noise2DPeriodic(w, h, x, y)
		wrapped := p.Noise2DPeriodic(w, h, wrapCoord(x, w), wrapCoord(y, h))
		if neg != wrapped {
			t.Errorf("(%v,%v): negative query %v != wrapped query %v", x, y, neg, wrapped)
		}
	}
}

// Test_PerlinNoise2DPeriodic_Determinism checks that repeated queries and
// separate generators with the same seed always agree.
func Test_PerlinNoise2DPeriodic_Determinism(t *testing.T) {
	p1 := NewPerlin(2, 2, 5, seed)
	p2 := NewPerlin(2, 2, 5, seed)
	for i := 0; i < 100; i++ {
		x := float64(i%13)*0.5 - 3.0
		y := float64(i%7)*0.75 - 2.0
		a := p1.Noise2DPeriodic(6, 4, x, y)
		if b := p1.Noise2DPeriodic(6, 4, x, y); b != a {
			t.Fatalf("repeated query differs: %v != %v", b, a)
		}
		if c := p2.Noise2DPeriodic(6, 4, x, y); c != a {
			t.Fatalf("generator with same seed differs: %v != %v", c, a)
		}
	}
}

// Test_PerlinNoise2DPeriodic_HighFrequencyOctaves exercises octaves whose
// cell counts far exceed the 256-entry permutation table, where table wrap
// masking and torus modulo interact.
func Test_PerlinNoise2DPeriodic_HighFrequencyOctaves(t *testing.T) {
	p := NewPerlin(2, 2, 10, seed)
	const w, h = 6.0, 4.0
	// Integer query points keep bit-exact periodicity through every
	// octave, since wrapping and rescaling only move integer values.
	for i := 0; i < 30; i++ {
		x := float64(i)
		y := float64(i) * 2
		base := p.Noise2DPeriodic(w, h, x, y)
		if math.IsNaN(base) || math.IsInf(base, 0) {
			t.Fatalf("non-finite result at (%v,%v): %v", x, y, base)
		}
		if got := p.Noise2DPeriodic(w, h, x+w, y+h); got != base {
			t.Fatalf("integer query lost exact periodicity: %v != %v", got, base)
		}
	}
	// Fractional points stay periodic within floating-point rounding,
	// which only tightens as the octave contributions shrink.
	for i := 0; i < 30; i++ {
		x := float64(i) * 0.21
		y := float64(i) * 0.17
		base := p.Noise2DPeriodic(w, h, x, y)
		if got := p.Noise2DPeriodic(w, h, x+w, y+h); !closeEnough(got, base, 1e-12) {
			t.Fatalf("periodicity lost at high frequency: %v != %v", got, base)
		}
	}
	// Must not panic even for the deepest octave's large torus.
	_ = p.Noise2DPeriodic(w, h, 5.99, 3.99)
}

// Test_PerlinNoise2DPeriodic_InvalidParameters checks the defined handling of
// invalid period arguments.
func Test_PerlinNoise2DPeriodic_InvalidParameters(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	bad := []struct {
		width, height float64
	}{
		{0, 4},
		{6, 0},
		{-6, 4},
		{6, -4},
		{math.NaN(), 4},
		{6, math.NaN()},
		{math.Inf(1), 4},
		{6, math.Inf(-1)},
	}
	for _, c := range bad {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Noise2DPeriodic(%v, %v, ...) expected panic", c.width, c.height)
				}
			}()
			_ = p.Noise2DPeriodic(c.width, c.height, 1, 1)
		}()
	}
	// Non-finite coordinates are not programmer errors: they return NaN
	// deterministically instead of panicking.
	if got := p.Noise2DPeriodic(6, 4, math.NaN(), 1); !math.IsNaN(got) {
		t.Errorf("NaN x: expected NaN, got %v", got)
	}
	if got := p.Noise2DPeriodic(6, 4, 1, math.Inf(1)); !math.IsNaN(got) {
		t.Errorf("Inf y: expected NaN, got %v", got)
	}
}

// Test_PerlinNoise2DPeriodic_ZeroOctaves is a defined edge case: with no
// octaves the sum is the zero function and remains periodic.
func Test_PerlinNoise2DPeriodic_ZeroOctaves(t *testing.T) {
	p := NewPerlin(2, 2, 0, seed)
	if got := p.Noise2DPeriodic(6, 4, 3.5, -2.25); got != 0 {
		t.Errorf("zero octaves: expected 0, got %v", got)
	}
}

func Benchmark_PerlinNoise1D(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	for n := 0; n < b.N; n++ {
		p.Noise1D(10)
	}
}

func Benchmark_PerlinNoise2D(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	for n := 0; n < b.N; n++ {
		p.Noise2D(10, 10)
	}
}
func Benchmark_PerlinNoise3D(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	for n := 0; n < b.N; n++ {
		p.Noise3D(10, 10, 10)
	}
}

func Benchmark_PerlinNoise2DPeriodic(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	for n := 0; n < b.N; n++ {
		p.Noise2DPeriodic(6, 4, 1.25, 2.75)
	}
}
