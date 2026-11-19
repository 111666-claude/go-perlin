package perlin

import (
	"errors"
	"math"
)

// Ridged multifractal tuning limits.
const (
	// ridgedLacunarity is the frequency multiplier applied between layers.
	ridgedLacunarity = 2.0
	// ridgedOffset turns gradient noise into a ridge signal: offset - |noise|.
	ridgedOffset = 1.0
	// RidgedMaxOctaves bounds the number of accumulated noise layers so that
	// every result stays finite and cheap to evaluate.
	RidgedMaxOctaves = 32
	// ridgedMaxCoord is the largest absolute noise coordinate sampled before
	// an error is returned. It keeps every layer inside the exact integer
	// range of the underlying gradient noise and prevents overflow.
	ridgedMaxCoord = 1e15
)

// Parameter and coordinate validation errors returned by the ridged samplers.
var (
	ErrInvalidOctaves     = errors.New("perlin: ridged octaves must be between 1 and RidgedMaxOctaves")
	ErrInvalidFrequency   = errors.New("perlin: ridged frequency must be finite and greater than zero")
	ErrInvalidPersistence = errors.New("perlin: ridged persistence must be finite and in the range (0, 1]")
	ErrInvalidGain        = errors.New("perlin: ridged gain must be finite and non-negative")
	ErrInvalidCoordinate  = errors.New("perlin: ridged coordinates must be finite and within the supported domain")
)

// RidgedOptions configures a ridged multifractal height field.
//
// The field is built from the existing 2D gradient noise: every layer turns
// the noise into a sharp ridge with (1-|noise|)^2 and the ridge feedback
// (Gain) lets strong ridges suppress the detail accumulated in the next
// layer, producing continuous crease lines instead of rounded hills.
type RidgedOptions struct {
	// Octaves is the number of layered noise frequencies (layers).
	// Must be in [1, RidgedMaxOctaves].
	Octaves int
	// Frequency is the base spatial frequency of the first layer.
	// Must be finite and strictly positive.
	Frequency float64
	// Persistence is the amplitude decay (attenuation) applied between
	// layers. Must be finite and in the range (0, 1].
	Persistence float64
	// Gain controls the ridge feedback weight: sharp ridges suppress
	// neighbouring detail in the next layer. The feedback weight is
	// always clamped to 1, so an arbitrarily large gain stays bounded.
	// Note that a gain of zero clears the weight after the first layer,
	// so the field collapses to its single base-frequency layer.
	Gain float64
}

// DefaultRidgedOptions returns terrain-oriented defaults: six layers, unit
// base frequency, 0.5 amplitude decay and a feedback gain of 2.
func DefaultRidgedOptions() RidgedOptions {
	return RidgedOptions{
		Octaves:     6,
		Frequency:   1.0,
		Persistence: 0.5,
		Gain:        2.0,
	}
}

// Validate checks that every parameter is finite and within its legal range.
// The ridged samplers call it themselves, so calling it directly is only
// needed when validating parameters ahead of time.
func (o RidgedOptions) Validate() error {
	if o.Octaves < 1 || o.Octaves > RidgedMaxOctaves {
		return ErrInvalidOctaves
	}
	if !isFinite(o.Frequency) || o.Frequency <= 0 {
		return ErrInvalidFrequency
	}
	if !isFinite(o.Persistence) || o.Persistence <= 0 || o.Persistence > 1 {
		return ErrInvalidPersistence
	}
	if !isFinite(o.Gain) || o.Gain < 0 {
		return ErrInvalidGain
	}
	return nil
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// Ridged2DHeight samples the ridged multifractal height at (x, y).
//
// The returned height is non-negative and bounded by Octaves (each layer
// contributes at most 1 after the ridge squaring and weight clamping).
// Results are deterministic for a fixed generator seed and coordinates and
// the sampler only reads the generator tables, so it is safe for concurrent
// use.
func (p *Perlin) Ridged2DHeight(opts RidgedOptions, x, y float64) (float64, error) {
	return p.ridged2D(opts, x, y)
}

// ridged2D is the shared height evaluation used by the height, slope and
// normal samplers.
func (p *Perlin) ridged2D(opts RidgedOptions, x, y float64) (float64, error) {
	if err := opts.Validate(); err != nil {
		return 0, err
	}
	if !isFinite(x) || !isFinite(y) {
		return 0, ErrInvalidCoordinate
	}

	frequency := opts.Frequency
	amplitude := 1.0
	weight := 1.0
	sum := 0.0

	for i := 0; i < opts.Octaves; i++ {
		sx := x * frequency
		sy := y * frequency
		// Reject coordinates that would leave the exact integer domain of
		// the underlying gradient noise (overflow or lost fraction bits).
		if math.Abs(sx) > ridgedMaxCoord || math.Abs(sy) > ridgedMaxCoord {
			return 0, ErrInvalidCoordinate
		}

		// Gradient noise is bounded by sqrt(2), so 1-|noise| lies in
		// (-1, 1] and its square is always in [0, 1].
		signal := ridgedOffset - math.Abs(p.noise2([2]float64{sx, sy}))
		signal *= signal
		signal *= weight

		// Ridge feedback: strong ridges reduce the weight of the next
		// layer. Clamping keeps the result bounded for any gain.
		weight = math.Min(1.0, signal*opts.Gain)

		sum += signal * amplitude
		amplitude *= opts.Persistence
		frequency *= ridgedLacunarity
	}
	return sum, nil
}

// Ridged2DSlope returns the height field gradient (dh/dx, dh/dy) at (x, y),
// estimated with adaptive central finite differences. Near the edge of the
// supported domain it falls back to a one-sided difference.
func (p *Perlin) Ridged2DSlope(opts RidgedOptions, x, y float64) (float64, float64, error) {
	h0, err := p.ridged2D(opts, x, y)
	if err != nil {
		return 0, 0, err
	}
	eps := slopeEpsilon(x, y)

	gx, err := centralSlope(func(v float64) (float64, error) {
		return p.ridged2D(opts, v, y)
	}, x, eps, h0)
	if err != nil {
		return 0, 0, err
	}
	gy, err := centralSlope(func(v float64) (float64, error) {
		return p.ridged2D(opts, x, v)
	}, y, eps, h0)
	if err != nil {
		return 0, 0, err
	}
	return gx, gy, nil
}

// Ridged2DNormal returns the unit upward surface normal (nx, ny, nz) at
// (x, y), treating the sampled height as the z coordinate of a mesh vertex.
// A perfectly flat patch yields (0, 0, 1).
func (p *Perlin) Ridged2DNormal(opts RidgedOptions, x, y float64) (float64, float64, float64, error) {
	gx, gy, err := p.Ridged2DSlope(opts, x, y)
	if err != nil {
		return 0, 0, 0, err
	}
	nx, ny, nz := -gx, -gy, 1.0
	length := math.Sqrt(nx*nx + ny*ny + nz*nz)
	return nx / length, ny / length, nz / length, nil
}

// slopeEpsilon scales the finite-difference step with the coordinate
// magnitude so that adding the step does not get swallowed by floating
// point precision at large coordinates.
func slopeEpsilon(x, y float64) float64 {
	m := math.Max(math.Abs(x), math.Abs(y))
	if m < 1 {
		m = 1
	}
	return 1e-6 * m
}

// centralSlope differentiates sample around v with step eps, falling back to
// a forward or backward difference when one side leaves the valid domain.
func centralSlope(sample func(float64) (float64, error), v, eps, h0 float64) (float64, error) {
	hi, errHi := sample(v + eps)
	lo, errLo := sample(v - eps)
	switch {
	case errHi == nil && errLo == nil:
		return (hi - lo) / (2 * eps), nil
	case errHi == nil:
		return (hi - h0) / eps, nil
	case errLo == nil:
		return (h0 - lo) / eps, nil
	default:
		return 0, errHi
	}
}
