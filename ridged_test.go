package perlin

import (
	"errors"
	"math"
	"testing"
)

// validRidgedOptions returns deterministic test parameters with a
// comfortably interior coordinate domain.
func validRidgedOptions() RidgedOptions {
	return RidgedOptions{
		Octaves:     5,
		Frequency:   1.5,
		Persistence: 0.5,
		Gain:        2.0,
	}
}

func TestRidgedOptions_Validate(t *testing.T) {
	ok := validRidgedOptions()
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}

	cases := []struct {
		name string
		opts RidgedOptions
		want error
	}{
		{"zero octaves", RidgedOptions{0, 1, 0.5, 2}, ErrInvalidOctaves},
		{"negative octaves", RidgedOptions{-1, 1, 0.5, 2}, ErrInvalidOctaves},
		{"too many octaves", RidgedOptions{RidgedMaxOctaves + 1, 1, 0.5, 2}, ErrInvalidOctaves},
		{"zero frequency", RidgedOptions{4, 0, 0.5, 2}, ErrInvalidFrequency},
		{"negative frequency", RidgedOptions{4, -1, 0.5, 2}, ErrInvalidFrequency},
		{"NaN frequency", RidgedOptions{4, math.NaN(), 0.5, 2}, ErrInvalidFrequency},
		{"infinite frequency", RidgedOptions{4, math.Inf(1), 0.5, 2}, ErrInvalidFrequency},
		{"zero persistence", RidgedOptions{4, 1, 0, 2}, ErrInvalidPersistence},
		{"persistence above one", RidgedOptions{4, 1, 1.01, 2}, ErrInvalidPersistence},
		{"negative persistence", RidgedOptions{4, 1, -0.5, 2}, ErrInvalidPersistence},
		{"NaN persistence", RidgedOptions{4, 1, math.NaN(), 2}, ErrInvalidPersistence},
		{"negative gain", RidgedOptions{4, 1, 0.5, -1}, ErrInvalidGain},
		{"NaN gain", RidgedOptions{4, 1, 0.5, math.NaN()}, ErrInvalidGain},
		{"infinite gain", RidgedOptions{4, 1, 0.5, math.Inf(1)}, ErrInvalidGain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.opts.Validate(), tc.want) {
				t.Fatalf("Validate() = %v, want %v", tc.opts.Validate(), tc.want)
			}
		})
	}
}

func TestRidged2DHeight_InvalidCoordinates(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()

	for _, c := range [][2]float64{
		{math.NaN(), 0},
		{0, math.NaN()},
		{math.Inf(1), 0},
		{math.Inf(-1), 0},
		{0, math.Inf(1)},
	} {
		if _, err := p.Ridged2DHeight(opts, c[0], c[1]); !errors.Is(err, ErrInvalidCoordinate) {
			t.Errorf("Ridged2DHeight(%v, %v): err = %v, want ErrInvalidCoordinate", c[0], c[1], err)
		}
	}

	// A coordinate whose highest layer leaves the supported domain is an
	// error rather than silent overflow.
	if _, err := p.Ridged2DHeight(opts, ridgedMaxCoord, 0); !errors.Is(err, ErrInvalidCoordinate) {
		t.Errorf("oversized coordinate: err = %v, want ErrInvalidCoordinate", err)
	}
	// The largest input whose final layer still lies on the domain
	// boundary is legal.
	maxInput := ridgedMaxCoord / opts.Frequency / math.Pow(ridgedLacunarity, float64(opts.Octaves-1))
	if _, err := p.Ridged2DHeight(opts, maxInput, 0); err != nil {
		t.Errorf("boundary coordinate rejected: %v", err)
	}
	if _, err := p.Ridged2DHeight(opts, maxInput*1.01, 0); !errors.Is(err, ErrInvalidCoordinate) {
		t.Errorf("coordinate past boundary: err = %v, want ErrInvalidCoordinate", err)
	}
}

// TestRidged2DHeight_SingleOctaveFormula pins the ridge signal to
// (1-|noise2(f*x, f*y)|)^2 for a single layer, which makes the whole
// implementation auditable against the underlying gradient noise.
func TestRidged2DHeight_SingleOctaveFormula(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := RidgedOptions{Octaves: 1, Frequency: 1.7, Persistence: 0.5, Gain: 2.0}

	for _, c := range [][2]float64{{0, 0}, {1.25, -3.5}, {12.7, 4.4}, {-8.1, 0.3}} {
		got, err := p.Ridged2DHeight(opts, c[0], c[1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		n := p.noise2([2]float64{c[0] * opts.Frequency, c[1] * opts.Frequency})
		want := ridgedOffset - math.Abs(n)
		want *= want
		if got != want {
			t.Errorf("at (%v, %v): got %v, want %v", c[0], c[1], got, want)
		}
	}
}

func TestRidged2DHeight_ZeroGainCollapsesToBaseLayer(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	base := validRidgedOptions()

	for _, c := range [][2]float64{{0.4, 0.9}, {7.2, -2.1}, {-5.5, 11.8}} {
		multi, err := p.Ridged2DHeight(RidgedOptions{
			Octaves: base.Octaves, Frequency: base.Frequency, Persistence: base.Persistence, Gain: 0,
		}, c[0], c[1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// In the canonical Musgrave formulation the zero gain clears the
		// feedback weight after the first layer, so every later layer
		// contributes nothing: the field equals its single base layer.
		n := p.noise2([2]float64{c[0] * base.Frequency, c[1] * base.Frequency})
		want := ridgedOffset - math.Abs(n)
		want *= want
		if multi != want {
			t.Errorf("at (%v, %v): got %v, want %v", c[0], c[1], multi, want)
		}
	}
}

func TestRidged2DHeight_DeterministicAndBounded(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	upper := 0.0
	amp := 1.0
	for i := 0; i < opts.Octaves; i++ {
		upper += amp
		amp *= opts.Persistence
	}

	first, err := p.Ridged2DHeight(opts, 3.7, 9.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 50; i++ {
		x := math.Sin(float64(i)*1.3) * 20
		y := math.Cos(float64(i)*0.7) * 20
		h, err := p.Ridged2DHeight(opts, x, y)
		if err != nil {
			t.Fatalf("unexpected error at (%v, %v): %v", x, y, err)
		}
		if !isFinite(h) {
			t.Fatalf("non-finite height at (%v, %v): %v", x, y, h)
		}
		if h < 0 || h > upper {
			t.Fatalf("height %v at (%v, %v) outside [0, %v]", h, x, y, upper)
		}
	}
	again, err := p.Ridged2DHeight(opts, 3.7, 9.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != again {
		t.Fatalf("non-deterministic result: %v vs %v", first, again)
	}
}

// TestRidged2DHeight_GainRunawayStaysBounded feeds an extreme gain: because
// the feedback weight is clamped to 1, the height must remain finite and
// within the standard amplitude envelope.
func TestRidged2DHeight_GainRunawayStaysBounded(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := RidgedOptions{Octaves: 8, Frequency: 1.0, Persistence: 0.5, Gain: 1e6}
	upper := 0.0
	amp := 1.0
	for i := 0; i < opts.Octaves; i++ {
		upper += amp
		amp *= opts.Persistence
	}
	for _, c := range [][2]float64{{2.3, 4.5}, {-6.7, 1.2}, {13.0, -9.4}} {
		h, err := p.Ridged2DHeight(opts, c[0], c[1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isFinite(h) || h < 0 || h > upper {
			t.Fatalf("unbounded height %v at (%v, %v), envelope [0, %v]", h, c[0], c[1], upper)
		}
	}
}

func TestRidged2DHeight_ParameterChanges(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	x, y := 2.6, -4.4

	base, err := p.Ridged2DHeight(opts, x, y)
	if err != nil {
		t.Fatal(err)
	}

	// Adding a layer only adds a non-negative term.
	more, err := p.Ridged2DHeight(RidgedOptions{
		Octaves: opts.Octaves + 1, Frequency: opts.Frequency, Persistence: opts.Persistence, Gain: opts.Gain,
	}, x, y)
	if err != nil {
		t.Fatal(err)
	}
	if more < base {
		t.Errorf("adding an octave lowered the height: %v -> %v", base, more)
	}

	// Feedback is observable somewhere on the grid (zero-gain differs
	// from the feedback variant), while at the sampled point a changed
	// gain must still produce a finite, in-range value.
	zero, err := p.Ridged2DHeight(RidgedOptions{
		Octaves: opts.Octaves, Frequency: opts.Frequency, Persistence: opts.Persistence, Gain: 0,
	}, x, y)
	if err != nil {
		t.Fatal(err)
	}
	if !isFinite(zero) {
		t.Fatalf("zero-gain height non-finite: %v", zero)
	}

	differs := false
	for gx := 0; gx < 32 && !differs; gx++ {
		for gy := 0; gy < 32; gy++ {
			withGain, _ := p.Ridged2DHeight(opts, float64(gx)*0.35, float64(gy)*0.35)
			noGain, _ := p.Ridged2DHeight(RidgedOptions{
				Octaves: opts.Octaves, Frequency: opts.Frequency, Persistence: opts.Persistence, Gain: 0,
			}, float64(gx)*0.35, float64(gy)*0.35)
			if withGain != noGain {
				differs = true
				break
			}
		}
	}
	if !differs {
		t.Error("gain parameter had no effect anywhere on the sample grid")
	}

	// Frequencies that differ by the lacunarity with one fewer octave
	// share their first layer; here we pin the frequency scaling by
	// rebuilding the field manually, layer by layer, with exact equality.
	hi := RidgedOptions{Octaves: opts.Octaves - 1, Frequency: opts.Frequency * ridgedLacunarity, Persistence: opts.Persistence, Gain: opts.Gain}
	hiH, err := p.Ridged2DHeight(hi, x, y)
	if err != nil {
		t.Fatal(err)
	}
	manual := 0.0
	amplitude := 1.0
	frequency := hi.Frequency
	weight := 1.0
	for i := 0; i < hi.Octaves; i++ {
		n := p.noise2([2]float64{x * frequency, y * frequency})
		signal := ridgedOffset - math.Abs(n)
		signal *= signal
		signal *= weight
		weight = math.Min(1.0, signal*hi.Gain)
		manual += signal * amplitude
		amplitude *= hi.Persistence
		frequency *= ridgedLacunarity
	}
	if hiH != manual {
		t.Errorf("frequency scaling mismatch: %v vs %v", hiH, manual)
	}
}

func TestRidged2DSlope_DirectionAndFiniteDifference(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()

	for _, c := range [][2]float64{{1.1, 2.2}, {-3.4, 5.6}, {8.8, -0.7}} {
		x, y := c[0], c[1]
		gx, gy, err := p.Ridged2DSlope(opts, x, y)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isFinite(gx) || !isFinite(gy) {
			t.Fatalf("non-finite slope at (%v, %v): (%v, %v)", x, y, gx, gy)
		}

		// Exact arithmetic pin: replicate the documented central
		// difference (same adaptive step) against the height sampler.
		eps := slopeEpsilon(x, y)
		hxp, _ := p.Ridged2DHeight(opts, x+eps, y)
		hxm, _ := p.Ridged2DHeight(opts, x-eps, y)
		hyp, _ := p.Ridged2DHeight(opts, x, y+eps)
		hym, _ := p.Ridged2DHeight(opts, x, y-eps)
		wantX := (hxp - hxm) / (2 * eps)
		wantY := (hyp - hym) / (2 * eps)
		if gx != wantX || gy != wantY {
			t.Errorf("at (%v, %v): slope (%v, %v), finite difference (%v, %v)", x, y, gx, gy, wantX, wantY)
		}

		// Direction check with an independent smaller step: a small step
		// up-gradient must raise the height and vice versa.
		mag := math.Hypot(gx, gy)
		if mag > 1e-9 {
			d := 1e-6 * math.Max(1.0, math.Max(math.Abs(x), math.Abs(y)))
			h0, _ := p.Ridged2DHeight(opts, x, y)
			h1, _ := p.Ridged2DHeight(opts, x+gx/mag*d, y+gy/mag*d)
			if h1 <= h0 {
				t.Errorf("up-gradient step lowered height at (%v, %v): %v -> %v", x, y, h0, h1)
			}
			h2, _ := p.Ridged2DHeight(opts, x-gx/mag*d, y-gy/mag*d)
			if h2 >= h0 {
				t.Errorf("down-gradient step raised height at (%v, %v): %v -> %v", x, y, h0, h2)
			}
		}
	}
}

// TestRidged2DSlope_DomainEdgeFallback verifies the one-sided fallback when
// the central step would leave the supported coordinate domain.
func TestRidged2DSlope_DomainEdgeFallback(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	// Place x so that x+eps leaves the domain while x-eps stays inside.
	xEdge := ridgedMaxCoord / opts.Frequency / math.Pow(ridgedLacunarity, float64(opts.Octaves-1))
	gx, gy, err := p.Ridged2DSlope(opts, xEdge, 0.5)
	if err != nil {
		t.Fatalf("edge slope rejected: %v", err)
	}
	if !isFinite(gx) || !isFinite(gy) {
		t.Fatalf("non-finite edge slope: (%v, %v)", gx, gy)
	}
}

func TestRidged2DSlope_PropagatesErrors(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	if _, _, err := p.Ridged2DSlope(validRidgedOptions(), math.NaN(), 0); !errors.Is(err, ErrInvalidCoordinate) {
		t.Fatalf("err = %v, want ErrInvalidCoordinate", err)
	}
	bad := validRidgedOptions()
	bad.Octaves = 0
	if _, _, err := p.Ridged2DSlope(bad, 0, 0); !errors.Is(err, ErrInvalidOctaves) {
		t.Fatalf("err = %v, want ErrInvalidOctaves", err)
	}
}

func TestRidged2DNormal_Properties(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	for _, c := range [][2]float64{{0.2, 0.4}, {5.5, -3.3}, {-7.7, 2.2}} {
		nx, ny, nz, err := p.Ridged2DNormal(opts, c[0], c[1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if length := math.Sqrt(nx*nx + ny*ny + nz*nz); math.Abs(length-1) > 1e-12 {
			t.Errorf("normal not unit length: %v", length)
		}
		if nz <= 0 {
			t.Errorf("normal is not upward-facing: nz=%v", nz)
		}
		gx, gy, err := p.Ridged2DSlope(opts, c[0], c[1])
		if err != nil {
			t.Fatal(err)
		}
		// The normal must be perpendicular to both surface tangents,
		// (1,0,gx) and (0,1,gy), which is equivalent to n = normalize(-g).
		if dot := nx*1 + ny*0 + nz*gx; math.Abs(dot) > 1e-10 {
			t.Errorf("normal not perpendicular to x-tangent: dot=%v", dot)
		}
		if dot := nx*0 + ny*1 + nz*gy; math.Abs(dot) > 1e-10 {
			t.Errorf("normal not perpendicular to y-tangent: dot=%v", dot)
		}
	}

	if _, _, _, err := p.Ridged2DNormal(opts, math.Inf(1), 0); !errors.Is(err, ErrInvalidCoordinate) {
		t.Fatalf("err = %v, want ErrInvalidCoordinate", err)
	}
}

// TestRidged2DHeight_RidgeContinuity walks a fixed-seed grid and checks
// that the highest heights form elongated, connected ridge lines rather
// than isolated peaks.
func TestRidged2DHeight_RidgeContinuity(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := RidgedOptions{Octaves: 6, Frequency: 0.12, Persistence: 0.5, Gain: 2.0}
	const size = 80

	grid := make([][]float64, size)
	maxH := math.Inf(-1)
	for i := 0; i < size; i++ {
		grid[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			h, err := p.Ridged2DHeight(opts, float64(i), float64(j))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			grid[i][j] = h
			if h > maxH {
				maxH = h
			}
		}
	}

	// Threshold at 85% of the observed maximum.
	threshold := maxH * 0.85
	visited := make([][]bool, size)
	for i := range visited {
		visited[i] = make([]bool, size)
	}

	var components, longRidges, ridgeCells int
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if visited[i][j] || grid[i][j] < threshold {
				continue
			}
			components++
			queue := [][2]int{{i, j}}
			visited[i][j] = true
			cells := 0
			for len(queue) > 0 {
				cell := queue[0]
				queue = queue[1:]
				cells++
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					ni, nj := cell[0]+d[0], cell[1]+d[1]
					if ni < 0 || ni >= size || nj < 0 || nj >= size || visited[ni][nj] || grid[ni][nj] < threshold {
						continue
					}
					visited[ni][nj] = true
					queue = append(queue, [2]int{ni, nj})
				}
			}
			ridgeCells += cells
			if cells >= 6 {
				longRidges++
			}
		}
	}

	if components < 2 {
		t.Fatalf("expected multiple ridge components, got %d", components)
	}
	if longRidges == 0 {
		t.Fatalf("no elongated ridge component (>=6 cells); components=%d, ridgeCells=%d", components, ridgeCells)
	}
	// Ridge cells must be concentrated: no single component may swallow
	// the whole grid, which would indicate an unbounded/plateau field.
	if ridgeCells > size*size/2 {
		t.Fatalf("ridge coverage %d/%d too large", ridgeCells, size*size)
	}
}

// Full-precision golden samples captured from the unchanged legacy
// generators (stored as IEEE-754 bit patterns).
var (
	goldenNoise1D = math.Float64frombits(4584696171040545664)
	goldenNoise2D = math.Float64frombits(13826911554461795426)
	goldenNoise3D = math.Float64frombits(13823248118789544794)
)

// TestRidged2DHeight_LatticeCusps verifies the mathematical signature of
// ridged terrain: at integer lattice points the underlying gradient noise
// is exactly zero, so the ridge signal reaches its peak and the gradient
// reverses across the point. A smooth fBm hill cannot produce such a
// gradient reversal; these cusps join into continuous sharp ridge lines
// along the grid.
func TestRidged2DHeight_LatticeCusps(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	opts := RidgedOptions{Octaves: 1, Frequency: 1.0, Persistence: 0.5, Gain: 2.0}
	const eps = 1e-6

	reversals := 0
	for _, x := range []float64{1, 2, 5, 10, 17, 23} {
		for _, y := range []float64{0.0, 3.0, 8.0} {
			if got := p.Noise2D(x, y); got != 0 {
				t.Fatalf("gradient noise not zero at lattice point (%v,%v): %v", x, y, got)
			}
			hC, err := p.Ridged2DHeight(opts, x, y)
			if err != nil {
				t.Fatal(err)
			}
			if hC != 1 {
				t.Fatalf("ridge not at peak 1 at lattice point (%v,%v): %v", x, y, hC)
			}
			hL, _ := p.Ridged2DHeight(opts, x-eps, y)
			hR, _ := p.Ridged2DHeight(opts, x+eps, y)
			dL, dR := (hC-hL)/eps, (hR-hC)/eps
			if dL == 0 || dR == 0 || math.Signbit(dL) == math.Signbit(dR) {
				t.Errorf("no gradient reversal at (%v,%v): dL=%v dR=%v", x, y, dL, dR)
			}
			reversals++
		}
	}
	if reversals == 0 {
		t.Fatal("no cusp checks ran")
	}
}

// TestLegacyNoiseOutputsUnchanged guards the requirement that adding ridged
// sampling must not alter the original Noise1D/2D/3D outputs. Values are
// full-precision golden samples for the existing generators.
func TestLegacyNoiseOutputsUnchanged(t *testing.T) {
	p := NewPerlin(2, 2, 3, seed)
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"Noise1D(10)", p.Noise1D(10), 0.0},
		{"Noise2D(10,10)", p.Noise2D(10, 10), 0.0},
		{"Noise3D(10,10,10)", p.Noise3D(10, 10, 10), 0.0},
		{"Noise1D(0.1)", p.Noise1D(0.1), goldenNoise1D},
		{"Noise2D(0.1,0.2)", p.Noise2D(0.1, 0.2), goldenNoise2D},
		{"Noise3D(0.1,0.2,0.3)", p.Noise3D(0.1, 0.2, 0.3), goldenNoise3D},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func BenchmarkRidged2DHeight(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	for n := 0; n < b.N; n++ {
		_, _ = p.Ridged2DHeight(opts, 1.23, 4.56)
	}
}

func BenchmarkRidged2DSlope(b *testing.B) {
	p := NewPerlin(2, 2, 3, seed)
	opts := validRidgedOptions()
	for n := 0; n < b.N; n++ {
		_, _, _ = p.Ridged2DSlope(opts, 1.23, 4.56)
	}
}
