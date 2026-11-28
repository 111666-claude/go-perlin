package perlin

import (
	"errors"
	"math"
)

// Pipe flow coefficient. With K=1 the maximum flux between two cells per
// step equals the free-surface height difference between them; any excess
// water is redistributed proportionally so water is never created.
const erosionFlowK = 1.0

// ErosionConfig configures a grid-based hydraulic erosion pass.
//
// The simulation is deterministic: it performs no random number generation
// and always visits neighbours in the fixed order north, east, south, west.
// Running Erode twice with the same configuration and heightmap therefore
// produces bit-identical results, and a Perlin generator created with a
// fixed seed keeps the whole terrain pipeline reproducible.
type ErosionConfig struct {
	// Steps is the number of simulated rainfall/erosion iterations.
	// Zero (or negative) makes Erode a no-op that returns the input
	// heights unchanged with zero accounting.
	Steps int

	// Precipitation is the water depth (in height units) added to every
	// cell at the beginning of each step. Must be finite and non-negative.
	Precipitation float64

	// ErosionRate is the fraction of a cell's transport capacity that is
	// picked up from its terrain when the flow carries too little
	// sediment. It is also used as the transport-capacity coefficient
	// (capacity = ErosionRate * flow * slope). Must be in [0, 1].
	ErosionRate float64

	// DepositionRate is the fraction of the excess sediment a flow
	// drops when it carries more sediment than its transport capacity
	// allows. Must be in [0, 1].
	DepositionRate float64

	// Evaporation is the fraction of standing water removed at the end
	// of every step (0..1). Zero disables evaporation. Evaporated water
	// is reported separately in ErosionResult and never carries
	// sediment away.
	Evaporation float64

	// BoundaryDrain selects how the grid border is treated.
	//
	// When false (the default) the border is a closed basin: water and
	// sediment cannot leave the grid, so terrain mass can only move
	// between cells and nothing is lost off-domain.
	//
	// When true every border cell gains a virtual lower neighbour below
	// the minimum terrain height of the input map. Flux crossing the
	// border is exported as off-domain water and sediment and reported
	// in ErosionResult, so every unit of mass is still accounted for.
	BoundaryDrain bool
}

// DefaultErosionConfig returns a conservative, non-destructive default
// configuration: 50 steps of rainfall with closed-basin borders.
func DefaultErosionConfig() ErosionConfig {
	return ErosionConfig{
		Steps:          50,
		Precipitation:  0.5,
		ErosionRate:    0.3,
		DepositionRate: 0.3,
		Evaporation:    0.1,
		BoundaryDrain:  false,
	}
}

func (c ErosionConfig) validate() error {
	if c.Steps < 0 {
		return errors.New("perlin: erosion Steps must be non-negative")
	}
	if !isFinite(c.Precipitation) || c.Precipitation < 0 {
		return errors.New("perlin: erosion Precipitation must be finite and non-negative")
	}
	if !isFinite(c.ErosionRate) || c.ErosionRate < 0 || c.ErosionRate > 1 {
		return errors.New("perlin: erosion ErosionRate must be finite and in [0, 1]")
	}
	if !isFinite(c.DepositionRate) || c.DepositionRate < 0 || c.DepositionRate > 1 {
		return errors.New("perlin: erosion DepositionRate must be finite and in [0, 1]")
	}
	if !isFinite(c.Evaporation) || c.Evaporation < 0 || c.Evaporation > 1 {
		return errors.New("perlin: erosion Evaporation must be finite and in [0, 1]")
	}
	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// ErosionResult holds the output of an Erode run.
//
// All slices have length Width*Height and use row-major indexing
// (index = y*Width + x). The input heightmap is never modified.
type ErosionResult struct {
	Width  int
	Height int

	// Heights is the new terrain height after erosion and deposition.
	Heights []float64
	// Water is the standing water depth remaining in each cell.
	Water []float64
	// Sediment is the suspended (still airborne) sediment load carried
	// by the water of each cell.
	Sediment []float64

	// Eroded accumulates the terrain height removed from each cell.
	// Always non-negative.
	Eroded []float64
	// Deposited accumulates the sediment height added to each cell.
	// Always non-negative.
	Deposited []float64

	// Rainfall is the total water added by precipitation over all steps.
	Rainfall float64
	// Evaporated is the total water removed by evaporation.
	Evaporated float64
	// ExportedWater is water that left the grid through a draining
	// boundary. Zero when BoundaryDrain is false.
	ExportedWater float64
	// ExportedSediment is sediment that left the grid through a
	// draining boundary. Zero when BoundaryDrain is false.
	ExportedSediment float64

	cfg ErosionConfig
}

// TerrainBalance returns the per-cell terrain mass ledger residual:
//
//	Height[i] - initialHeight[i] + Eroded[i] - Deposited[i]
//
// Every residual is zero (up to floating-point rounding) because terrain
// can only be converted into suspended sediment or rebuilt from it.
func (r *ErosionResult) TerrainBalance(initial []float64) (maxAbs float64, ok bool) {
	if len(initial) != len(r.Heights) {
		return math.NaN(), false
	}
	maxAbs = 0.0
	for i, h0 := range initial {
		res := r.Heights[i] - h0 + r.Eroded[i] - r.Deposited[i]
		if a := math.Abs(res); a > maxAbs {
			maxAbs = a
		}
	}
	return maxAbs, maxAbs <= 1e-9
}

// WaterBalance checks the global water ledger
//
//	standing + evaporated + exported == rainfall
//
// and the global sediment/terrain ledger
//
//	suspended + exported + deposited == eroded
//
// returning the largest absolute residual and whether both close within
// tolerance. No terrain mass is ever created or destroyed: off-domain
// exports are reported instead of silently disappearing.
func (r *ErosionResult) WaterBalance() (waterResidual, sedimentResidual float64, ok bool) {
	var standing, suspended, eroded, deposited float64
	for i := range r.Heights {
		standing += r.Water[i]
		suspended += r.Sediment[i]
		eroded += r.Eroded[i]
		deposited += r.Deposited[i]
	}
	waterResidual = standing + r.Evaporated + r.ExportedWater - r.Rainfall
	sedimentResidual = suspended + r.ExportedSediment + deposited - eroded
	waterTol := 1e-9 * math.Max(1.0, math.Abs(r.Rainfall))
	sedimentTol := 1e-9 * math.Max(1.0, math.Abs(eroded))
	ok = math.Abs(waterResidual) <= waterTol && math.Abs(sedimentResidual) <= sedimentTol
	return
}

// neighbour links a cell to one of its four neighbours. A target of -1
// denotes the virtual boundary sink used when BoundaryDrain is enabled.
type neighbour struct{ target, dir int }

// maxDownhillDiff returns the largest positive height difference between
// cell i and a real in-grid neighbour. The virtual drain (target -1) is
// ignored: it only removes mass, so it must not inflate transport capacity
// and let a cell erode more material than the terrain actually provides.
func maxDownhillDiff(i int, nbs []neighbour, heights []float64) float64 {
	maxDiff := 0.0
	for _, nb := range nbs {
		if nb.target == -1 {
			continue
		}
		if d := heights[i] - heights[nb.target]; d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff
}

// Erode runs the configured hydraulic erosion simulation over a finite
// heightmap and returns the new heights together with a full erosion and
// deposition accounting.
//
// height must be a row-major slice of width*height finite values. The
// input slice is never modified. A non-finite height, a dimension mismatch
// or an invalid configuration produces an error and leaves the input
// untouched.
func Erode(height []float64, width, heightDim int, cfg ErosionConfig) (*ErosionResult, error) {
	if width <= 0 || heightDim <= 0 {
		return nil, errors.New("perlin: erosion grid dimensions must be positive")
	}
	n := width * heightDim
	if len(height) != n {
		return nil, errors.New("perlin: erosion heightmap length does not match dimensions")
	}
	for _, h := range height {
		if !isFinite(h) {
			return nil, errors.New("perlin: erosion heightmap contains a non-finite height")
		}
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	res := &ErosionResult{
		Width:     width,
		Height:    heightDim,
		Heights:   append([]float64(nil), height...),
		Water:     make([]float64, n),
		Sediment:  make([]float64, n),
		Eroded:    make([]float64, n),
		Deposited: make([]float64, n),
		cfg:       cfg,
	}
	if cfg.Steps == 0 || (cfg.Precipitation == 0 && cfg.ErosionRate == 0 && cfg.DepositionRate == 0) {
		return res, nil
	}

	// Precompute the neighbour table. A target index of -1 marks the
	// virtual sink used when BoundaryDrain is enabled.
	neighbours := make([][]neighbour, n)
	minH := res.Heights[0]
	for i := 1; i < n; i++ {
		if res.Heights[i] < minH {
			minH = res.Heights[i]
		}
	}
	sinkSurface := minH - 1
	dirs := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} // N, E, S, W
	for y := 0; y < heightDim; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			for d, dd := range dirs {
				nx, ny := x+dd[0], y+dd[1]
				if nx < 0 || nx >= width || ny < 0 || ny >= heightDim {
					if cfg.BoundaryDrain {
						neighbours[i] = append(neighbours[i], neighbour{-1, d})
					}
					continue
				}
				neighbours[i] = append(neighbours[i], neighbour{ny*width + nx, d})
			}
		}
	}

	// Outgoing flux per cell per direction, recreated each step.
	flow := make([][4]float64, n)
	totalOut := make([]float64, n)
	totalIn := make([]float64, n)
	sedIn := make([]float64, n)

	for step := 0; step < cfg.Steps; step++ {
		// 1. Rainfall.
		if cfg.Precipitation > 0 {
			for i := range res.Water {
				res.Water[i] += cfg.Precipitation
			}
			res.Rainfall += cfg.Precipitation * float64(n)
		}

		for i := range flow {
			flow[i] = [4]float64{}
			totalOut[i] = 0
			totalIn[i] = 0
			sedIn[i] = 0
		}

		// 2. Flow routing (pipe model, Jacobi update). Every flux is
		// computed from the same pre-routing free-surface (terrain +
		// water) snapshot before any water is moved, so cell visit order
		// cannot change the result. Flux is proportional to the surface
		// difference and only runs toward strictly lower surfaces.
		for i := 0; i < n; i++ {
			surface := res.Heights[i] + res.Water[i]
			var demand float64
			for _, nb := range neighbours[i] {
				var otherSurface float64
				if nb.target == -1 {
					otherSurface = sinkSurface
				} else {
					otherSurface = res.Heights[nb.target] + res.Water[nb.target]
				}
				if diff := surface - otherSurface; diff > 0 {
					f := erosionFlowK * diff
					flow[i][nb.dir] = f
					demand += f
				}
			}
			if demand > res.Water[i] && demand > 0 {
				// Never route more water than the cell actually holds.
				scale := res.Water[i] / demand
				for d := range flow[i] {
					flow[i][d] *= scale
				}
				demand = res.Water[i]
			}
			totalOut[i] = demand
		}

		// 3. Erosion. A flow's transport capacity grows with flux and
		// slope; underloaded flows pick terrain up, capped by the
		// available terrain so a cell cannot erode below its neighbours.
		// The virtual drain is an outlet for mass, not terrain, so the
		// capacity slope only considers real in-grid neighbours.
		// This runs before routing so the freshly eroded sediment travels
		// with this step's flux instead of settling back in place.
		for i := 0; i < n; i++ {
			if totalOut[i] <= 0 || cfg.ErosionRate <= 0 {
				continue
			}
			maxDiff := maxDownhillDiff(i, neighbours[i], res.Heights)
			capacity := cfg.ErosionRate * totalOut[i] * maxDiff
			if capacity > res.Sediment[i] {
				amount := cfg.ErosionRate * (capacity - res.Sediment[i])
				if amount > maxDiff {
					amount = maxDiff
				}
				if amount > res.Heights[i] {
					amount = res.Heights[i]
				}
				if amount > 0 {
					res.Heights[i] -= amount
					res.Sediment[i] += amount
					res.Eroded[i] += amount
				}
			}
		}

		// 4. Route water and its suspended sediment (including the load
		// just eroded). Sediment entering a cell (or the sink) is split in
		// proportion to the water flux.
		for i := 0; i < n; i++ {
			for _, nb := range neighbours[i] {
				f := flow[i][nb.dir]
				if f <= 0 {
					continue
				}
				if nb.target == -1 {
					res.ExportedWater += f
					if totalOut[i] > 0 {
						// Sediment leaves in proportion to the water flux
						// over the water held before routing.
						res.ExportedSediment += res.Sediment[i] * f / res.Water[i]
					}
					continue
				}
				totalIn[nb.target] += f
				if totalOut[i] > 0 {
					sedIn[nb.target] += res.Sediment[i] * f / res.Water[i]
				}
			}
		}
		for i := 0; i < n; i++ {
			if totalOut[i] > 0 {
				res.Sediment[i] *= 1 - totalOut[i]/res.Water[i]
			}
			res.Water[i] += totalIn[i] - totalOut[i]
			res.Sediment[i] += sedIn[i]
			if res.Water[i] < 0 {
				res.Water[i] = 0
			}
			if res.Sediment[i] < 0 {
				res.Sediment[i] = 0
			}
		}

		// 5. Deposition. Overloaded or stagnant flows drop sediment onto
		// the terrain; on perfectly flat ground the full load settles.
		// Capacity uses real neighbours only; sediment carried toward the
		// drain is exported rather than forcing artificial deposition.
		for i := 0; i < n; i++ {
			if res.Sediment[i] <= 0 || cfg.DepositionRate <= 0 {
				continue
			}
			maxDiff := maxDownhillDiff(i, neighbours[i], res.Heights)
			capacity := cfg.ErosionRate * totalOut[i] * maxDiff
			var amount float64
			if res.Sediment[i] > capacity {
				amount = cfg.DepositionRate * (res.Sediment[i] - capacity)
			} else if totalOut[i] == 0 {
				amount = cfg.DepositionRate * res.Sediment[i]
			}
			if amount > res.Sediment[i] {
				amount = res.Sediment[i]
			}
			if amount > 0 {
				res.Heights[i] += amount
				res.Sediment[i] -= amount
				res.Deposited[i] += amount
			}
		}

		// 6. Evaporation removes water only; sediment stays in place.
		if cfg.Evaporation > 0 {
			for i := range res.Water {
				evap := cfg.Evaporation * res.Water[i]
				res.Water[i] -= evap
				res.Evaporated += evap
			}
		}

		// Defensive guard against any non-finite arithmetic.
		for i := 0; i < n; i++ {
			if !isFinite(res.Heights[i]) || !isFinite(res.Water[i]) || !isFinite(res.Sediment[i]) {
				return nil, errors.New("perlin: erosion produced a non-finite value")
			}
		}
	}

	return res, nil
}

// HeightMap2D samples Noise2D over a width*height grid. Coordinates are
// mapped through scale (sample point (x, y) becomes (x*scale, y*scale)),
// and offset shifts the sampled region. The returned slice is row-major
// and can be passed directly to Erode.
func (p *Perlin) HeightMap2D(width, height int, scale, offsetX, offsetY float64) ([]float64, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("perlin: heightmap dimensions must be positive")
	}
	if !isFinite(scale) || scale <= 0 {
		return nil, errors.New("perlin: heightmap scale must be finite and positive")
	}
	hm := make([]float64, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			hm[y*width+x] = p.Noise2D(offsetX+float64(x)*scale, offsetY+float64(y)*scale)
		}
	}
	return hm, nil
}
