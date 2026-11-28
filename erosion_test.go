package perlin

import (
	"math"
	"math/rand"
	"testing"
)

// approxEqual compares floats accounting for floating-point round-off.
func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol*math.Max(1.0, math.Max(math.Abs(a), math.Abs(b)))
}

// Test_Erode3x1RightwardOutflow is the canonical conservation scenario:
// heights [3,1,1] with closed borders, one rain step. Only the first cell
// may be eroded; its material must appear as sediment in a downstream cell.
func Test_Erode3x1RightwardOutflow(t *testing.T) {
	initial := []float64{3, 1, 1}
	cfg := ErosionConfig{
		Steps:          1,
		Precipitation:  1,
		ErosionRate:    1,
		DepositionRate: 1,
		Evaporation:    0,
		BoundaryDrain:  false,
	}
	res, err := Erode(initial, 3, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cell 2 cannot flow anywhere and cell 1 is flat relative to cell 2,
	// so neither can erode in step one; only cell 0 loses terrain.
	if res.Eroded[1] != 0 || res.Eroded[2] != 0 {
		t.Errorf("only the first cell may erode, got eroded = %v", res.Eroded)
	}
	if res.Eroded[0] <= 0 {
		t.Fatalf("expected the first cell to erode, got %v", res.Eroded)
	}
	// The first cell must actually be lowered by exactly its erosion.
	if !approxEqual(res.Heights[0], initial[0]-res.Eroded[0], 1e-12) {
		t.Errorf("eroded cell height mismatch: %f != %f", res.Heights[0], initial[0]-res.Eroded[0])
	}
	// Cells 1 and 2 were not eroded; cell 1 may receive deposited
	// sediment from the incoming flow, cell 2 stays untouched this step.
	if res.Heights[2] != initial[2] {
		t.Errorf("flat downstream cell must stay unchanged, got %f", res.Heights[2])
	}
	if res.Heights[1] < initial[1] {
		t.Errorf("cell 1 can only gain deposition, got %f", res.Heights[1])
	}
	// Every eroded unit is either deposited downstream or still
	// suspended when the step ends - never destroyed.
	var sediment, deposited float64
	for i := range res.Sediment {
		sediment += res.Sediment[i]
		deposited += res.Deposited[i]
	}
	if !approxEqual(sediment+deposited, res.Eroded[0], 1e-12) {
		t.Errorf("eroded mass %f not preserved as sediment %f + deposit %f",
			res.Eroded[0], sediment, deposited)
	}
	// Closed borders: no off-domain loss.
	if res.ExportedWater != 0 || res.ExportedSediment != 0 {
		t.Errorf("closed basin must not export mass, got water %f sediment %f",
			res.ExportedWater, res.ExportedSediment)
	}
	// Cell-by-cell terrain ledger: h' - h + eroded - deposited == 0.
	if maxRes, ok := res.TerrainBalance(initial); !ok {
		t.Errorf("per-cell terrain balance broken, max residual %g", maxRes)
	}
	// Global water and sediment ledgers.
	if w, s, ok := res.WaterBalance(); !ok {
		t.Errorf("global balance broken: water residual %g, sediment residual %g", w, s)
	}
	// Input must not be mutated.
	if initial[0] != 3 || initial[1] != 1 || initial[2] != 1 {
		t.Errorf("Erode mutated the input heightmap: %v", initial)
	}
}

// Test_Erode3x1Drain reports off-domain loss when the border drains, and
// the loss must still close the global conservation ledgers.
func Test_Erode3x1Drain(t *testing.T) {
	initial := []float64{3, 2, 1}
	cfg := ErosionConfig{
		Steps:          1,
		Precipitation:  1,
		ErosionRate:    1,
		DepositionRate: 1,
		Evaporation:    0,
		BoundaryDrain:  true,
	}
	res, err := Erode(initial, 3, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExportedWater <= 0 {
		t.Errorf("expected exported water at the draining border, got %f", res.ExportedWater)
	}
	if res.ExportedSediment < 0 {
		t.Errorf("exported sediment must be non-negative, got %f", res.ExportedSediment)
	}
	if maxRes, ok := res.TerrainBalance(initial); !ok {
		t.Errorf("per-cell terrain balance broken, max residual %g", maxRes)
	}
	if w, s, ok := res.WaterBalance(); !ok {
		t.Errorf("global balance broken: water residual %g, sediment residual %g", w, s)
	}
}

// Test_ErodeDrainSedimentEventuallyExports runs enough steps for eroded
// material to reach the draining border, proving no terrain mass vanishes.
func Test_ErodeDrainSedimentEventuallyExports(t *testing.T) {
	initial := []float64{5, 4, 3, 2, 1}
	cfg := ErosionConfig{
		Steps:          20,
		Precipitation:  1,
		ErosionRate:    0.5,
		DepositionRate: 0.5,
		Evaporation:    0,
		BoundaryDrain:  true,
	}
	res, err := Erode(initial, 5, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var eroded float64
	for _, v := range res.Eroded {
		eroded += v
	}
	if eroded == 0 {
		t.Fatal("expected erosion on a descending slope")
	}
	if res.ExportedSediment <= 0 {
		t.Errorf("expected some eroded sediment to leave the grid, got %f", res.ExportedSediment)
	}
	if _, _, ok := res.WaterBalance(); !ok {
		t.Error("global balance broken with draining border")
	}
}

// Test_ErodeFlatMapDoesNothing: on level ground there is no flow, hence no
// erosion and no terrain change; rain simply ponds and evaporates.
func Test_ErodeFlatMapDoesNothing(t *testing.T) {
	initial := []float64{2, 2, 2, 2}
	cfg := DefaultErosionConfig()
	cfg.Steps = 10
	cfg.BoundaryDrain = false
	res, err := Erode(initial, 2, 2, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range initial {
		if res.Heights[i] != initial[i] {
			t.Errorf("flat terrain changed at cell %d: %f -> %f", i, initial[i], res.Heights[i])
		}
		if res.Eroded[i] != 0 || res.Deposited[i] != 0 {
			t.Errorf("flat terrain must not erode or deposit at cell %d", i)
		}
	}
	if w, _, ok := res.WaterBalance(); !ok {
		t.Errorf("water ledger broken on flat map, residual %g", w)
	}
}

// Test_ErodeSingleCell: a lone cell has no neighbours. With closed borders
// nothing moves at all; with draining borders it may flow into the virtual
// sink, but every eroded unit is reported through the ledgers.
func Test_ErodeSingleCell(t *testing.T) {
	initial := []float64{7}
	cfg := ErosionConfig{
		Steps:          3,
		Precipitation:  1,
		ErosionRate:    1,
		DepositionRate: 1,
		Evaporation:    0,
		BoundaryDrain:  false,
	}
	res, err := Erode(initial, 1, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Heights[0] != 7 || res.Eroded[0] != 0 || res.Deposited[0] != 0 {
		t.Errorf("single closed cell must keep its terrain: %+v", res)
	}
	if !approxEqual(res.Water[0], 3, 1e-12) {
		t.Errorf("single closed cell should pond all rain, got water %f", res.Water[0])
	}

	cfg.BoundaryDrain = true
	res, err = Erode(initial, 1, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With a draining border the lone cell flows downhill into the
	// virtual sink and erodes; every removed unit must be reported as
	// exported sediment or deposited back, never lost.
	if res.Heights[0] > 7 {
		t.Errorf("draining single cell must not gain terrain: %f", res.Heights[0])
	}
	if res.ExportedWater <= 0 {
		t.Errorf("draining single cell should export water, got %f", res.ExportedWater)
	}
	if res.ExportedSediment < 0 {
		t.Errorf("exported sediment must be non-negative, got %f", res.ExportedSediment)
	}
	if maxRes, ok := res.TerrainBalance(initial); !ok {
		t.Errorf("per-cell terrain balance broken, residual %g", maxRes)
	}
	if _, _, ok := res.WaterBalance(); !ok {
		t.Error("global balance broken for draining single cell")
	}
}

// Test_ErodeZeroStepsAndNoRain are explicit no-ops returning the input.
func Test_ErodeZeroStepsAndNoRain(t *testing.T) {
	initial := []float64{1, 2, 3, 4}
	for _, cfg := range []ErosionConfig{
		{Steps: 0, Precipitation: 1},
		{Steps: 5, Precipitation: 0, ErosionRate: 0, DepositionRate: 0},
	} {
		res, err := Erode(initial, 2, 2, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i := range initial {
			if res.Heights[i] != initial[i] {
				t.Errorf("no-op run changed height at %d", i)
			}
			if res.Water[i] != 0 || res.Sediment[i] != 0 ||
				res.Eroded[i] != 0 || res.Deposited[i] != 0 {
				t.Errorf("no-op run produced accounting at cell %d", i)
			}
		}
	}
}

// Test_ErodeNoMassCreated enforces physical bounds over many steps:
// exported plus suspended sediment can never exceed the eroded total
// (minus what was deposited back), and a closed basin keeps its total
// terrain mass exactly.
func Test_ErodeNoMassCreated(t *testing.T) {
	initial := []float64{5, 4, 3, 2, 1}
	var initialMass float64
	for _, h := range initial {
		initialMass += h
	}
	for _, drain := range []bool{false, true} {
		cfg := ErosionConfig{
			Steps:          30,
			Precipitation:  1,
			ErosionRate:    0.9,
			DepositionRate: 0.7,
			Evaporation:    0.05,
			BoundaryDrain:  drain,
		}
		res, err := Erode(initial, 5, 1, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var eroded, deposited, suspended float64
		for i := range initial {
			eroded += res.Eroded[i]
			deposited += res.Deposited[i]
			suspended += res.Sediment[i]
		}
		// Sediment in existence cannot exceed what was removed from terrain.
		if suspended+res.ExportedSediment > eroded+1e-12 {
			t.Errorf("drain=%v: sediment created out of nothing: %g > %g",
				drain, suspended+res.ExportedSediment, eroded)
		}
		// Deposited terrain cannot exceed what was eroded.
		if deposited > eroded+1e-12 {
			t.Errorf("drain=%v: deposited %g exceeds eroded %g", drain, deposited, eroded)
		}
		// Closed basin: terrain plus the sediment still suspended in its
		// water equals the initial mass - eroded material is only parked
		// in suspension, never lost.
		if !drain {
			var mass float64
			for _, h := range res.Heights {
				mass += h
			}
			if !approxEqual(mass+suspended, initialMass, 1e-12) {
				t.Errorf("closed basin lost mass: terrain+sediment %f != %f",
					mass+suspended, initialMass)
			}
			if res.ExportedWater != 0 || res.ExportedSediment != 0 {
				t.Error("closed basin must not export mass")
			}
		}
		if _, _, ok := res.WaterBalance(); !ok {
			t.Errorf("drain=%v: global ledger broken", drain)
		}
	}
}

// Test_ErodeRejectsInvalidInputs covers non-finite heights and extreme or
// invalid parameters.
func Test_ErodeRejectsInvalidInputs(t *testing.T) {
	good := []float64{1, 2, 3, 4}
	cases := []struct {
		name   string
		height []float64
		w, h   int
		cfg    ErosionConfig
	}{
		{"nan height", []float64{1, math.NaN(), 3, 4}, 2, 2, DefaultErosionConfig()},
		{"+inf height", []float64{1, math.Inf(1), 3, 4}, 2, 2, DefaultErosionConfig()},
		{"-inf height", []float64{1, math.Inf(-1), 3, 4}, 2, 2, DefaultErosionConfig()},
		{"length mismatch", good, 3, 2, DefaultErosionConfig()},
		{"zero width", good, 0, 2, DefaultErosionConfig()},
		{"negative steps", good, 2, 2, ErosionConfig{Steps: -1, Precipitation: 1}},
		{"nan rain", good, 2, 2, ErosionConfig{Steps: 1, Precipitation: math.NaN()}},
		{"negative rain", good, 2, 2, ErosionConfig{Steps: 1, Precipitation: -1}},
		{"erosion > 1", good, 2, 2, ErosionConfig{Steps: 1, Precipitation: 1, ErosionRate: 1.5}},
		{"deposition < 0", good, 2, 2, ErosionConfig{Steps: 1, Precipitation: 1, DepositionRate: -0.1}},
		{"evaporation > 1", good, 2, 2, ErosionConfig{Steps: 1, Precipitation: 1, Evaporation: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Erode(tc.height, tc.w, tc.h, tc.cfg); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// Test_ErodeExtremeParameters stays finite and conservative under huge
// rainfall and maximal rates.
func Test_ErodeExtremeParameters(t *testing.T) {
	initial := []float64{100, 75, 50, 25, 10}
	cfg := ErosionConfig{
		Steps:          10,
		Precipitation:  1e6,
		ErosionRate:    1,
		DepositionRate: 1,
		Evaporation:    1,
		BoundaryDrain:  true,
	}
	res, err := Erode(initial, 5, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range res.Heights {
		if !isFinite(v) || !isFinite(res.Water[i]) || !isFinite(res.Sediment[i]) {
			t.Fatalf("non-finite output at cell %d: %+v", i, res)
		}
		if v < 0 {
			t.Errorf("terrain went negative at cell %d: %f", i, v)
		}
	}
	if maxRes, ok := res.TerrainBalance(initial); !ok {
		t.Errorf("terrain balance broken under extreme params, residual %g", maxRes)
	}
	if w, s, ok := res.WaterBalance(); !ok {
		t.Errorf("global balance broken under extreme params: %g %g", w, s)
	}
}

// Test_ErodeDeterministic requires bit-identical output across runs and
// proves the original sampler is untouched by erosion.
func Test_ErodeDeterministic(t *testing.T) {
	p := NewPerlin(2, 2, 3, 123)
	hm, err := p.HeightMap2D(12, 9, 0.15, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := DefaultErosionConfig()
	r1, err := Erode(hm, 12, 9, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := Erode(hm, 12, 9, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range r1.Heights {
		if r1.Heights[i] != r2.Heights[i] ||
			r1.Water[i] != r2.Water[i] ||
			r1.Sediment[i] != r2.Sediment[i] ||
			r1.Eroded[i] != r2.Eroded[i] ||
			r1.Deposited[i] != r2.Deposited[i] {
			t.Fatalf("non-deterministic output at cell %d", i)
		}
	}
	// The sampler must still produce its original values.
	if got := p.Noise2D(10, 10); got != 0.0 {
		t.Errorf("Noise2D sampler changed after erosion: %f", got)
	}
}

// Test_ErodeRandomGridsConserveMass fuzzes random 2D heightmaps and
// configurations, checking both ledgers on every cell and globally.
func Test_ErodeRandomGridsConserveMass(t *testing.T) {
	rng := rand.New(rand.NewSource(424242))
	for iter := 0; iter < 40; iter++ {
		w, h := 1+rng.Intn(8), 1+rng.Intn(8)
		initial := make([]float64, w*h)
		for i := range initial {
			initial[i] = rng.Float64()*20 - 5 // signed heights, like Noise2D
		}
		cfg := ErosionConfig{
			Steps:          1 + rng.Intn(8),
			Precipitation:  rng.Float64() * 2,
			ErosionRate:    rng.Float64(),
			DepositionRate: rng.Float64(),
			Evaporation:    rng.Float64(),
			BoundaryDrain:  rng.Intn(2) == 0,
		}
		res, err := Erode(initial, w, h, cfg)
		if err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if maxRes, ok := res.TerrainBalance(initial); !ok {
			t.Fatalf("iter %d: per-cell terrain balance broken, residual %g", iter, maxRes)
		}
		if wRes, sRes, ok := res.WaterBalance(); !ok {
			t.Fatalf("iter %d: global balance broken, water %g sediment %g", iter, wRes, sRes)
		}
		// Accounting slices keep a pure sign.
		for i := range initial {
			if res.Eroded[i] < 0 || res.Deposited[i] < 0 || res.Water[i] < 0 || res.Sediment[i] < 0 {
				t.Fatalf("iter %d: negative accounting at cell %d", iter, i)
			}
		}
		// Closed basins can never export anything.
		if !cfg.BoundaryDrain && (res.ExportedWater != 0 || res.ExportedSediment != 0) {
			t.Fatalf("iter %d: closed basin exported mass", iter)
		}
	}
}

// Test_ErodeEvaporationBalance checks that evaporated water is reported and
// the remaining water still closes the rainfall ledger.
func Test_ErodeEvaporationBalance(t *testing.T) {
	initial := []float64{0, 0}
	cfg := ErosionConfig{
		Steps:          1,
		Precipitation:  1,
		ErosionRate:    0,
		DepositionRate: 0,
		Evaporation:    0.25,
		BoundaryDrain:  false,
	}
	res, err := Erode(initial, 2, 1, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEqual(res.Evaporated, 0.5, 1e-12) {
		t.Errorf("expected 0.5 evaporated, got %f", res.Evaporated)
	}
	if !approxEqual(res.Water[0]+res.Water[1], 1.5, 1e-12) {
		t.Errorf("expected 1.5 standing water, got %v", res.Water)
	}
	if w, _, ok := res.WaterBalance(); !ok {
		t.Errorf("water ledger broken, residual %g", w)
	}
}
