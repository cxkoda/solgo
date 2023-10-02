// Package memconv implements "type punning" of raw memory, reinterpreting one
// type's representation as another. Note that this is *pointer* type
// conversion, not human-intuitive type conversion: for example, a float64
// reinterpreted as a uint64 would simply expose the IEEE-75 floating-point
// representation.
package memconv

import "unsafe"

// cast implements the behaviour described by the CastPtr() function without any
// checks of the size of From and To; it MUST NOT be exported as said checks are
// enforced by the exported functions that rely on it.
func cast[From any, To any](x *From) *To {
	return (*To)(unsafe.Pointer(x))
}

// A B64 is a contiguous region of 64 bits.
type B64 interface {
	~int64 | ~uint64 | ~float64 | ~complex64 | ~[8]byte
}

// CastPtr returns x, interpreted as a pointer to a different type, using the
// same underlying memory; i.e. a change to the value pointed to by the
// incoming pointer will result in a change to the value pointed to by the
// returned pointer, and vice versa.
func CastPtr[From B64, To B64](x *From) *To {
	return cast[From, To](x)
}

// Cast is equivalent to CastPtr but it returns a copy.
func Cast[From B64, To B64](x *From) To {
	return *cast[From, To](x)
}

// A B32 is a contiguous region of 32 bits.
type B32 interface {
	~int32 | ~uint32 | ~float32 | ~[4]byte
}

// CastPtr32 is the 32-bit equivalent of CastPtr().
func CastPtr32[From B32, To B32](x *From) *To {
	return cast[From, To](x)
}

// Cast32 is the 32-bit equivalent of Cast().
func Cast32[From B32, To B32](x *From) To {
	return *cast[From, To](x)
}
