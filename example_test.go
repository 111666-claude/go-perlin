package perlin_test

import (
	"fmt"
	"math/rand"

	"github.com/aquilax/go-perlin"
)

const (
	alpha       = 2.
	beta        = 2.
	n           = 3
	seed  int64 = 100
)

func ExampleNewPerlinRandSource() {
	p := perlin.NewPerlinRandSource(alpha, beta, n, rand.NewSource(seed))
	for x := 0.; x < 3; x++ {
		fmt.Printf("%0.0f\t%0.4f\n", x, p.Noise1D(x/10))
	}
	// Output:
	// 0	0.0000
	// 1	-0.1670
	// 2	-0.2011
}

func ExamplePerlin_Noise1D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 3; x++ {
		fmt.Printf("%0.0f\t%0.4f\n", x, p.Noise1D(x/10))
	}
	// Output:
	// 0	0.0000
	// 1	-0.1670
	// 2	-0.2011
}

func ExamplePerlin_Noise2D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 2; x++ {
		for y := 0.; y < 2; y++ {
			fmt.Printf("%0.0f\t%0.0f\t%0.4f\n", x, y, p.Noise2D(x/10, y/10))
		}
	}
	// Output:
	// 0	0	0.0000
	// 0	1	-0.0060
	// 1	0	0.1230
	// 1	1	0.0625
}

func ExamplePerlin_Noise3D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 2; x++ {
		for y := 0.; y < 2; y++ {
			for z := 0.; z < 2; z++ {
				fmt.Printf("%0.0f\t%0.0f\t%0.0f\t%0.4f\n", x, y, z, p.Noise3D(x/10, y/10, z/10))
			}
		}
	}
	// Output:
	// 0	0	0	0.0000
	// 0	0	1	0.1881
	// 0	1	0	0.1384
	// 0	1	1	0.2507
	// 1	0	0	-0.0416
	// 1	0	1	0.1232
	// 1	1	0	0.0536
	// 1	1	1	0.1826
}

func ExamplePerlin_Ridged2DHeight() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	opts := perlin.DefaultRidgedOptions()
	for _, c := range [][2]float64{{0, 0}, {0.5, 0.5}, {1, 1}} {
		h, err := p.Ridged2DHeight(opts, c[0], c[1])
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%0.1f\t%0.1f\t%0.4f\n", c[0], c[1], h)
	}
	// Output:
	// 0.0	0.0	1.9688
	// 0.5	0.5	1.8096
	// 1.0	1.0	1.9688
}

func ExamplePerlin_Ridged2DNormal() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	opts := perlin.DefaultRidgedOptions()
	nx, ny, nz, err := p.Ridged2DNormal(opts, 0.5, 0.5)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%0.4f\t%0.4f\t%0.4f\n", nx, ny, nz)
	// Output:
	// 0.5649	0.2483	0.7870
}
