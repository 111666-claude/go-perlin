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
		fmt.Printf("%0.0f;%0.4f\n", x, p.Noise1D(x/10))
	}
	// Output:
	// 0;0.0000
	// 1;-0.0086
	// 2;-0.0017
}

func ExamplePerlin_Noise1D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 3; x++ {
		fmt.Printf("%0.0f;%0.4f\n", x, p.Noise1D(x/10))
	}
	// Output:
	// 0;0.0000
	// 1;-0.0086
	// 2;-0.0017
}

func ExamplePerlin_Noise2D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 2; x++ {
		for y := 0.; y < 2; y++ {
			fmt.Printf("%0.0f;%0.0f;%0.4f\n", x, y, p.Noise2D(x/10, y/10))
		}
	}
	// Output:
	// 0;0;0.0000
	// 0;1;-0.2002
	// 1;0;-0.3389
	// 1;1;-0.5045
}

func ExamplePerlin_Noise3D() {
	p := perlin.NewPerlin(alpha, beta, n, seed)
	for x := 0.; x < 2; x++ {
		for y := 0.; y < 2; y++ {
			for z := 0.; z < 2; z++ {
				fmt.Printf("%0.0f;%0.0f;%0.0f;%0.4f\n", x, y, z, p.Noise3D(x/10, y/10, z/10))
			}
		}
	}
	// Output:
	// 0;0;0;0.0000
	// 0;0;1;0.2616
	// 0;1;0;-0.0755
	// 0;1;1;0.2020
	// 1;0;0;-0.2138
	// 1;0;1;0.0616
	// 1;1;0;-0.2208
	// 1;1;1;0.0304
}

func ExampleErode() {
	// A noisy ridgeline sampled from Noise2D with a fixed seed, tilted so
	// the rainfall has somewhere to flow.
	p := perlin.NewPerlin(alpha, beta, n, seed)
	noise, err := p.HeightMap2D(4, 1, 0.3, 0, 0)
	if err != nil {
		panic(err)
	}
	heightMap := make([]float64, len(noise))
	for i, v := range noise {
		heightMap[i] = v + float64(3-i) // 3, 2, 1, 0 tilt
	}

	cfg := perlin.ErosionConfig{
		Steps:          10,
		Precipitation:  0.5,
		ErosionRate:    0.3,
		DepositionRate: 0.3,
		Evaporation:    0.1,
		BoundaryDrain:  false,
	}
	result, err := perlin.Erode(heightMap, 4, 1, cfg)
	if err != nil {
		panic(err)
	}

	// The simulation is deterministic: the same config and heightmap
	// always produce the same eroded terrain and closed ledgers.
	_, _, ok := result.WaterBalance()
	fmt.Println("ledgers balanced:", ok)
	for i, h := range result.Heights {
		fmt.Printf("%d: %0.4f (eroded %0.4f, deposited %0.4f)\n",
			i, h, result.Eroded[i], result.Deposited[i])
	}
	// Output:
	// ledgers balanced: true
	// 0: 2.3491 (eroded 0.9351, deposited 0.2842)
	// 1: 0.9517 (eroded 1.1456, deposited 0.6129)
	// 2: 0.6775 (eroded 0.6327, deposited 0.6406)
	// 3: 0.2748 (eroded 0.0000, deposited 0.3196)
}
