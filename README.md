## GO Perlin Noise generator [![Go Reference](https://pkg.go.dev/badge/github.com/aquilax/go-perlin.svg)](https://pkg.go.dev/github.com/aquilax/go-perlin)

Adapted for go from GEGL http://git.gnome.org/browse/gegl/tree/operations/common/perlin


Example: perlin.Noise1D

    alpha = 2
    beta = 2
    n = 3

![PerlinNoise1D](http://i.imgur.com/Kplg5.png)

Using perlin.Noise2D with termbox to generate terrain in terminal:

![PerlinNoise2D](http://i.imgur.com/vPi3n.png)

## Ridged multifractal height sampling

Plain fractal (fBm) noise produces rounded hills. For mountain terrain with
continuous sharp ridge lines, use the ridged multifractal samplers, built on
top of the existing 2D gradient noise:

```go
p := perlin.NewPerlin(2, 2, 3, 100)
opts := perlin.RidgedOptions{
    Octaves:     6,     // number of noise layers
    Frequency:   1.0,   // base spatial frequency (> 0)
    Persistence: 0.5,   // amplitude decay between layers, in (0, 1]
    Gain:        2.0,   // ridge feedback weight (>= 0, clamped internally)
}

h, err := p.Ridged2DHeight(opts, x, y)           // vertex height, always bounded
gx, gy, err := p.Ridged2DSlope(opts, x, y)       // height field gradient
nx, ny, nz, err := p.Ridged2DNormal(opts, x, y)  // unit upward surface normal
```

The ridge signal `(1-|noise|)^2` turns every zero crossing of the gradient
noise into a sharp crease; the gain parameter lets strong ridges suppress
detail in the next layer. Results are deterministic for a fixed seed and
coordinates, non-finite or out-of-domain coordinates and invalid parameters
return sentinel errors, and the original `Noise1D`/`Noise2D`/`Noise3D`
outputs are unchanged.
