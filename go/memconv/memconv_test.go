package memconv

import (
	"fmt"
	"math"
	"testing"
)

func ExampleCast() {
	posInt := int64(42)
	posUint := Cast[int64, uint64](&posInt)
	negInt := -posInt

	const format = "%[1]T %#9[1]b => %[2]T %#064[2]b"
	fmt.Printf(format+"\n", posInt, posUint)
	fmt.Printf(format+" (two's complement)\n\n", negInt, Cast[int64, uint64](&negInt))

	// Note that, unlike CastPtr(), this only changes posInt as posUint is a
	// copy.
	posInt++
	fmt.Printf("%[1]T %[1]d != %[2]T %[2]d (copy unless using CastPtr())\n\n", posInt, posUint)

	oneFP := 1.
	fmt.Printf("%[1]T %.3[1]f => %[2]T %#064[2]b (IEEE-754 representation)\n", oneFP, Cast[float64, uint64](&oneFP))

	// Output:
	// int64  0b101010 => uint64 0b0000000000000000000000000000000000000000000000000000000000101010
	// int64 -0b101010 => uint64 0b1111111111111111111111111111111111111111111111111111111111010110 (two's complement)
	//
	// int64 43 != uint64 42 (copy unless using CastPtr())
	//
	// float64 1.000 => uint64 0b0011111111110000000000000000000000000000000000000000000000000000 (IEEE-754 representation)
}

func ExampleCastPtr() {
	posInt := int64(42)
	posUint := CastPtr[int64, uint64](&posInt)

	// Note that this changes both posInt and posUint because they share the
	// same underlying memory.
	posInt++
	fmt.Printf("%[1]T %[1]d => %[2]T %[2]d (shared memory on increment)\n", posInt, *posUint)

	// Output: int64 43 => uint64 43 (shared memory on increment)
}

func Example() {
	// Reimplementation of
	// https://github.com/id-Software/Quake-III-Arena/blob/dbe4ddb10315479fc00086f08e25d968b4b43c49/code/game/q_math.c#L552C1-L552C1
	// with original comments for the lolz.
	quakeInverseSqrt := func(number float32) float32 {
		x2 := number * 0.5
		y := number

		i := CastPtr32[float32, int32](&y) // evil floating point bit level hacking
		*i = 0x5f3759df - (*i >> 1)        // what the fuck?

		for j := 0; j < 3; j++ {
			y *= 1.5 - (x2 * y * y) // jth iteration
		}

		return y
	}

	for _, x := range []float32{2, 3, 4, 1. / 9.} {
		fmt.Printf("%.3f => %.6f vs %.6f\n", x, quakeInverseSqrt(x), 1/math.Sqrt(float64(x)))
	}

	// Output:
	// 2.000 => 0.707107 vs 0.707107
	// 3.000 => 0.577350 vs 0.577350
	// 4.000 => 0.500000 vs 0.500000
	// 0.111 => 3.000000 vs 3.000000
}

func TestFloat32Bits(t *testing.T) {
	// The math.Float{32,64}bits() and inverse functions function identically
	// to Cast{32,64}() respectively.
	for _, f := range []float32{0, 1, 2, math.Pi, 1e20} {
		if got, want := Cast32[float32, uint32](&f), math.Float32bits(f); got != want {
			t.Errorf("Cast32[float32,uint32](%f) got %#032b, want %#032b", f, got, want)
		}
	}
}

/**
 * DANGER
 */

// If you are thinking of exporting the cast() function, please look at this
// first. A user will do it—I promise you.
var _ = cast[int32, int64]

// Seriously, are you sure you want to make cast() available to everyone?
var _ = cast[uint8, [1024]byte]

/**
 * HERE BE DRAGONS
 */
