## GO Perlin Noise generator [![Go Reference](https://pkg.go.dev/badge/github.com/aquilax/go-perlin.svg)](https://pkg.go.dev/github.com/aquilax/go-perlin)

Adapted for go from GEGL http://git.gnome.org/browse/gegl/tree/operations/common/perlin


Example: perlin.Noise1D

    alpha = 2
    beta = 2
    n = 3

![PerlinNoise1D](http://i.imgur.com/Kplg5.png)

Using perlin.Noise2D with termbox to generate terrain in terminal:

![PerlinNoise2D](http://i.imgur.com/vPi3n.png)

## Tileable noise

`Noise2DPeriodic(width, height, x, y)` generates 2D noise that repeats
every `width` units on the x axis and every `height` units on the y axis,
so a rectangle sampled over `[0,width) x [0,height)` tiles seamlessly in
both directions (and across both at once). The value and its local
variation stay continuous across every edge and at the four corners.

Periods may be fractional and do not have to match whole lattice cells;
negative coordinates are wrapped deterministically. The periods must be
positive and finite.

```go
p := perlin.NewPerlin(2, 2, 3, 100)
const width, height = 6.0, 4.0
v := p.Noise2DPeriodic(width, height, x, y)
// The next tile returns exactly the same value.
_ = p.Noise2DPeriodic(width, height, x+width, y+height)
```
