// Package rand is a port of Mozzi's mozzi_rand.h / internal/mozzi_rand_p.h:
// a fast xorshift96 PRNG, much cheaper than a general-purpose random source,
// intended for use inside updateAudio()/updateControl() without glitching
// the audio output.
//
// It is deliberately not "math/rand": that pulls in machinery this port
// doesn't need on a microcontroller, and importing it can be enough to drag
// float64 support into a TinyGo binary. This is a direct algorithmic port
// instead, seeded with upstream's exact default seed values so sketches
// ported from Mozzi see the same sequences.
package rand

// x, y, z hold the xorshift96 generator state. Upstream: MozziGuts.hpp.
var (
	x uint32 = 132456789
	y uint32 = 362436069
	z uint32 = 521288629
)

// Xorshift96 returns the next 32 bit pseudo-random number.
// Marsaglia, George (2003). Xorshift RNGs. http://www.jstatsoft.org/v08/i14/xorshift.pdf
func Xorshift96() uint32 {
	// period 2^96-1
	x ^= x << 16
	x ^= x >> 5
	x ^= x << 1

	t := x
	x = y
	y = z
	z = t ^ x ^ y
	return z
}

// Seed re-seeds Xorshift96. Useful if you want different random sequences
// on each run, e.g. by seeding from an unconnected ADC pin's noise.
func Seed(seed uint32) { x = seed }

// Int8 returns a random signed byte in [minval, maxval).
//
// The range maxval-minval is computed in int32, like upstream's promotion
// to int: in int8 it overflows for ranges wider than 127 (e.g. -100..100).
func Int8(minval, maxval int8) int8 {
	return int8((int32(uint8(Xorshift96()))*(int32(maxval)-int32(minval)))>>8) + minval
}

// Int8Max returns a random signed byte in [0, maxval).
func Int8Max(maxval int8) int8 {
	return int8((int32(uint8(Xorshift96())) * int32(maxval)) >> 8)
}

// Uint8 returns a random unsigned byte in [minval, maxval).
func Uint8(minval, maxval uint8) uint8 {
	return uint8((uint32(uint8(Xorshift96()))*uint32(maxval-minval))>>8) + minval
}

// Uint8Max returns a random unsigned byte in [0, maxval).
func Uint8Max(maxval uint8) uint8 {
	return uint8((uint32(uint8(Xorshift96())) * uint32(maxval)) >> 8)
}

// Int16 returns a random int in [minval, maxval), matching upstream's 16 bit range.
//
// As in Int8, the range is computed in int32 so it can't overflow int16.
func Int16(minval, maxval int16) int16 {
	return int16(((Xorshift96()&0xFFFF)*uint32(int32(maxval)-int32(minval)))>>16) + minval
}

// Int16Max returns a random int in [0, maxval).
func Int16Max(maxval int16) int16 {
	return int16(((Xorshift96() & 0xFFFF) * uint32(maxval)) >> 16)
}

// Uint16 returns a random unsigned int in [minval, maxval).
func Uint16(minval, maxval uint16) uint16 {
	return uint16(((Xorshift96()&0xFFFF)*uint32(maxval-minval))>>16) + minval
}

// Uint16Max returns a random unsigned int in [0, maxval).
func Uint16Max(maxval uint16) uint16 {
	return uint16(((Xorshift96() & 0xFFFF) * uint32(maxval)) >> 16)
}

// MidiNote returns a random value in [0, 127], suitable as a MIDI note number.
func MidiNote() uint8 {
	return uint8(Xorshift96()) >> 1
}
