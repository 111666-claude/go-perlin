## GO Perlin Noise generator [![Go Reference](https://pkg.go.dev/badge/github.com/aquilax/go-perlin.svg)](https://pkg.go.dev/github.com/aquilax/go-perlin)

Adapted for go from GEGL http://git.gnome.org/browse/gegl/tree/operations/common/perlin


Example: perlin.Noise1D

    alpha = 2
    beta = 2
    n = 3

![PerlinNoise1D](http://i.imgur.com/Kplg5.png)

Using perlin.Noise2D with termbox to generate terrain in terminal:

![PerlinNoise2D](http://i.imgur.com/vPi3n.png)

## Hydraulic erosion

A raw noise heightmap has one-cell spikes and pits. `Erode` applies a
deterministic grid-based hydraulic erosion (pipe model) pass that softens
ridgelines without flattening them:

```go
p := perlin.NewPerlin(2, 2, 3, 100)
heightMap, _ := p.HeightMap2D(64, 64, 0.1, 0, 0) // Noise2D on a finite grid

result, err := perlin.Erode(heightMap, 64, 64, perlin.ErosionConfig{
    Steps:          50,   // rainfall iterations
    Precipitation:  0.5,  // water depth added per cell per step
    ErosionRate:    0.3,  // pickup / transport-capacity rate
    DepositionRate: 0.3,  // sediment drop rate
    Evaporation:    0.1,  // fraction of water removed per step
    BoundaryDrain:  false,
})
```

Properties:

- **Deterministic** — no random numbers; a fixed seed and config give
  bit-identical terrain. The input heightmap and the `Perlin` sampler are
  never modified.
- **Mass conserving** — water only moves between cells, evaporates or is
  exported through an optional draining boundary (`BoundaryDrain`). Terrain
  is converted into suspended sediment and back; nothing is created or
  destroyed. `ErosionResult.TerrainBalance` and `WaterBalance` verify both
  ledgers per cell and globally.
- **Explicit edge handling** — flat maps do not move, a single-cell grid is
  well defined, non-finite heights and out-of-range parameters return an
  error, and off-domain loss is reported as `ExportedWater`/`ExportedSediment`
  rather than disappearing.

