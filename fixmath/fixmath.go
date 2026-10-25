// Package fixmath is a port of Mozzi's mozzi_fixmath.h / mozzi_fixmath.cpp:
// fast, integer-only fixed-point arithmetic types and conversions.
//
// Fixed point is often the best choice for fast audio code on small MCUs,
// and TinyGo/RP2040 is no exception: the Cortex-M0+ core in the Pico has no
// hardware float unit, so float32 math is emulated in software and is
// noticeably slower than integer math in a hot updateAudio() loop.
//
// Naming follows upstream: QmnN is a fixed-point type with m integer bits
// and n fractional bits (Q for signed, "Qmn0" style also covers plain
// integers). See https://github.com/sensorium/Mozzi/blob/main/mozzi_fixmath.h
//
// Note: upstream also ships a newer, type-safe FixMath library (UFix/SFix
// template classes). This port sticks to the older, plain-typedef API
// mirrored here, which covers everything the rest of this port needs.
package fixmath

// Q0n7 is a signed fractional number using 7 fractional bits: -0.5 to 0.496.
type Q0n7 = int8

// Q7n0 is a plain signed int8: -128 to 127.
type Q7n0 = int8

// Q0n8 is an unsigned fractional number using 8 fractional bits: 0.0 to 0.996.
type Q0n8 = uint8

// Q8n0 is a plain uint8: 0.0 to 255.0.
type Q8n0 = uint8

// Q0n16 is an unsigned fractional number using 16 fractional bits.
type Q0n16 = uint16

// Q0n31 is a signed number using 0 integer bits and 31 fractional bits (stored unsigned).
type Q0n31 = uint32

// Q7n8 is a signed fractional number: 7 integer bits, 8 fractional bits.
type Q7n8 = int16

// Q3n13 is an unsigned fractional number: 3 integer bits, 13 fractional bits.
type Q3n13 = uint16

// Q1n14 is a signed fractional number: 1 integer bit, 14 fractional bits.
type Q1n14 = int16

// Q15n0 is a plain signed int16.
type Q15n0 = int16

// Q8n8 is an unsigned fractional number: 8 integer bits, 8 fractional bits.
type Q8n8 = uint16

// Q0n15 is a signed fractional number using 15 fractional bits.
type Q0n15 = int16

// Q1n15 is an unsigned fractional number: 1 integer bit, 15 fractional bits.
type Q1n15 = uint16

// Q16n0 is a plain uint16.
type Q16n0 = uint16

// Q23n8 is a signed fractional number: 23 integer bits, 8 fractional bits.
type Q23n8 = int32

// Q15n16 is a signed fractional number: 15 integer bits, 16 fractional bits.
type Q15n16 = int32

// Q31n0 is a plain signed int32.
type Q31n0 = int32

// Q32n0 is a plain uint32.
type Q32n0 = uint32

// Q0n32 is an unsigned fractional number using 32 fractional bits.
type Q0n32 = uint32

// Q8n24 is an unsigned fractional number: 8 integer bits, 24 fractional bits.
type Q8n24 = uint32

// Q24n8 is an unsigned fractional number: 24 integer bits, 8 fractional bits.
type Q24n8 = uint32

// Q16n16 is an unsigned fractional number: 16 integer bits, 16 fractional bits.
type Q16n16 = uint32

// Fixed representations of 1.0 in various formats, to save runtime calculation.
const (
	Q0n7Fix1   Q0n7   = 127
	Q7n8Fix1   Q7n8   = 256
	Q8n8Fix1   Q8n8   = 256
	Q23n8Fix1  Q23n8  = 256
	Q1n14Fix1  Q1n14  = 16384
	Q1n15Fix1  Q1n15  = 32768
	Q16n16Fix1 Q16n16 = 65536
	Q0n15Fix1  Q0n15  = 32767
	Q0n16Fix1  Q0n16  = 65535
	Q15n0Fix1  Q15n0  = 16384
	Q15n16Fix1 Q15n16 = 65536
	Q8n24Fix1  Q8n24  = 16777216
	Q0n32Fix1  Q0n32  = 4294967295

	Q16n16PI    Q16n16 = 205887
	Q3n13TwoPI  Q3n13  = 411775 & 0xFFFF // upstream casts 411775 (2*PI in Q3n13) to a 16 bit Q3n13, truncating it
	Q16n16TwoPI Q16n16 = 411775

	// Low15Bits masks the lower 15 bits of a Q1n15 number.
	Low15Bits Q1n15 = 32767
)

// --- Float <-> fixed conversions ---
//
// To convert a float to Qm.n: multiply by 2^n and round to the nearest int.
// To convert Qm.n to a float: treat as an integer, then multiply by 2^-n.

func FloatToQ0n7(a float32) Q0n7     { return Q0n7(a * 256) }
func FloatToQ0n8(a float32) Q0n8     { return Q0n8(a * 256) }
func FloatToQ7n8(a float32) Q7n8     { return Q7n8(a * 256) }
func FloatToQ8n8(a float32) Q8n8     { return Q8n8(a * 256) }
func FloatToQ1n14(a float32) Q1n14   { return Q1n14(a * 16384) }
func FloatToQ1n15(a float32) Q1n15   { return Q1n15(a * 32768) }
func FloatToQ8n24(a float32) Q8n24   { return Q8n24(int32(a * 16777216)) }
func FloatToQ23n8(a float32) Q23n8   { return Q23n8(a * 256) }
func FloatToQ24n8(a float32) Q24n8   { return Q24n8(int32(a * 256)) }
func FloatToQ16n16(a float32) Q16n16 { return Q16n16(int32(a * 65536)) }
func FloatToQ0n16(a float32) Q0n16   { return Q0n16(uint32(a * 65536)) }
func FloatToQ15n16(a float32) Q15n16 { return Q15n16(a * 65536) }

func Q0n7ToQ1n14(a Q0n7) Q1n14   { return Q1n14(a) << 7 }
func Q0n7ToQ15n16(a Q0n7) Q15n16 { return Q15n16(a) << 8 }
func Q0n7ToFloat(a Q0n7) float32 { return float32(a) / 256 }

func Q0n8ToQ1n15(a Q0n8) Q1n15   { return Q1n15(a) << 7 }
func Q0n8ToQ8n8(a Q0n8) Q8n8     { return Q8n8(a) << 8 }
func Q0n8ToQ8n24(a Q0n8) Q8n24   { return Q8n24(a) << 16 }
func Q0n8ToQ24n8(a Q0n8) Q24n8   { return Q24n8(a) << 8 }
func Q0n8ToQ15n16(a Q0n8) Q15n16 { return Q15n16(a) << 8 }
func Q0n8ToQ16n16(a Q0n8) Q16n16 { return Q16n16(a) << 8 }
func Q0n8ToFloat(a Q0n8) float32 { return float32(a) / 256 }

func Q7n0ToQ7n8(a Q7n0) Q7n8     { return Q7n8(a) << 8 }
func Q7n0ToQ15n16(a Q7n0) Q15n16 { return Q15n16(a) << 16 }

func Q8n0ToQ7n8(a Q8n0) Q7n8     { return Q7n8(a) << 8 }
func Q8n0ToQ8n8(a Q8n0) Q8n8     { return Q8n8(a) << 8 }
func Q8n0ToQ15n16(a Q8n0) Q15n16 { return Q15n16(a) << 16 }
func Q8n0ToQ16n16(a Q8n0) Q16n16 { return Q16n16(a) << 16 }

func Q7n8ToQ7n0(a Q7n8) Q7n0     { return Q7n0(a >> 8) }
func Q7n8ToQ15n16(a Q7n8) Q15n16 { return Q15n16(a) << 8 }
func Q7n8ToFloat(a Q7n8) float32 { return float32(a) / 256 }

func Q8n8ToQ8n0(a Q8n8) Q8n0     { return Q8n0(a >> 8) }
func Q8n8ToQ16n16(a Q8n8) Q16n16 { return Q16n16(a) << 8 }
func Q8n8ToFloat(a Q8n8) float32 { return float32(a) / 256 }

func Q1n14ToQ0n7(a Q1n14) Q0n7     { return Q0n7(a >> 7) }
func Q1n14ToFloat(a Q1n14) float32 { return float32(a) / 16384 }

func Q1n15ToQ0n8(a Q1n15) Q0n8     { return Q0n8(a >> 7) }
func Q1n15ToFloat(a Q1n15) float32 { return float32(a) / 32768 }

func Q0n16ToFloat(a Q0n16) float32 { return float32(a) / 65536 }

func Q15n0ToQ15n16(a Q15n0) Q15n16 { return Q15n16(a) << 16 }
func Q16n0ToQ15n16(a Q16n0) Q15n16 { return Q15n16(a) << 16 }
func Q16n0ToQ23n8(a Q16n0) Q23n8   { return Q23n8(a) << 8 }
func Q16n0ToQ24n8(a Q16n0) Q24n8   { return Q24n8(a) << 8 }
func Q16n0ToQ16n16(a Q16n0) Q16n16 { return Q16n16(a) << 16 }
func Q16n0ToFloat(a Q16n0) float32 { return float32(a) }

func Q8n24ToQ0n8(a Q8n24) Q0n8     { return Q0n8(a >> 16) }
func Q8n24ToFloat(a Q8n24) float32 { return float32(a) / 16777216 }

func Q23n8ToQ31n0(a Q23n8) Q31n0   { return Q31n0(a >> 8) }
func Q23n8ToQ16n0(a Q23n8) Q16n0   { return Q16n0(a >> 8) }
func Q23n8ToQ15n0(a Q23n8) Q15n0   { return Q15n0(a >> 8) }
func Q23n8ToQ7n8(a Q23n8) Q7n8     { return Q7n8(a) }
func Q23n8ToFloat(a Q23n8) float32 { return float32(a) / 256 }

func Q24n8ToQ0n8(a Q24n8) Q0n8     { return Q0n8(a) }
func Q24n8ToQ16n0(a Q24n8) Q16n0   { return Q16n0(a >> 8) }
func Q24n8ToQ32n0(a Q24n8) Q32n0   { return Q32n0(a >> 8) }
func Q24n8ToQ16n16(a Q24n8) Q16n16 { return Q16n16(a) << 8 }
func Q24n8ToFloat(a Q24n8) float32 { return float32(a) / 256 }

func Q15n16ToQ0n8(a Q15n16) Q0n8     { return Q0n8(a >> 8) }
func Q15n16ToQ8n0(a Q15n16) Q8n0     { return Q8n0(a >> 16) }
func Q15n16ToQ15n0(a Q15n16) Q15n0   { return Q15n0(a >> 16) }
func Q15n16ToQ7n8(a Q15n16) Q7n8     { return Q7n8(a >> 8) }
func Q15n16ToQ8n8(a Q15n16) Q8n8     { return Q8n8(a >> 8) }
func Q15n16ToQ23n8(a Q15n16) Q23n8   { return Q23n8(a >> 8) }
func Q15n16ToFloat(a Q15n16) float32 { return float32(a) / 65536 }

func Q16n16ToQ0n8(a Q16n16) Q0n8     { return Q0n8(a >> 8) }
func Q16n16ToQ8n8(a Q16n16) Q8n8     { return Q8n8(a >> 16) }
func Q16n16ToQ16n0(a Q16n16) Q16n0   { return Q16n0(a >> 16) }
func Q16n16ToQ24n8(a Q16n16) Q24n8   { return Q24n8(a >> 8) }
func Q16n16ToFloat(a Q16n16) float32 { return float32(a) / 65536 }

// Q7n8Mult is a fast fixed-point multiply for Q7n8 fractional numbers.
func Q7n8Mult(a, b Q7n8) Q7n8 {
	return Q7n8((int32(a) * int32(b)) >> 8)
}

// Ipow is exponentiation by squaring; dangerously overflow-prone, as upstream.
func Ipow(base, exp int16) int16 {
	result := int16(1)
	for exp != 0 {
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
	}
	return result
}

// Q16n16Pow2 is a fast, approximate replacement for pow(2,x), where x is a
// Q8n8 fractional exponent. Less accurate than math.Pow, but cheap.
func Q16n16Pow2(exponent Q8n8) Q16n16 {
	q := uint8(exponent >> 8) // integer part
	n := uint8(exponent)      // fractional part
	return (Q16n16(Q8n8Fix1) << q) * (Q16n16(Q8n8Fix1) + Q16n16(n))
}

// Uint8Mod is a fast uint8 modulus (branch-free alternative avoided upstream
// for AVR division cost; kept for parity, plain '%' is fine on RP2040).
func Uint8Mod(n, d uint8) uint8 {
	for n >= d {
		n -= d
	}
	return n
}

// Uint8Div is a fast uint8 division, see Uint8Mod.
func Uint8Div(n, d uint8) uint8 {
	var q uint8
	for n >= d {
		n -= d
		q++
	}
	return q
}

var uint8RndSeed uint8

// Uint8Rnd is upstream's tiny 1-byte PRNG. Not for cryptographic use, and,
// per upstream's own warning, not very random -- prefer package mozzi/rand.
func Uint8Rnd(min, max uint8) uint8 {
	uint8RndSeed = 21*uint8RndSeed + 21
	return min + Uint8Mod(uint8RndSeed, max-1)
}

// Isqrt16 computes the integer square root of a uint16.
func Isqrt16(n uint16) uint16 {
	var root, remainder, place uint16
	remainder = n
	place = 0x4000
	for place > remainder {
		place >>= 2
	}
	for place != 0 {
		if remainder >= root+place {
			remainder -= root + place
			root += place << 1
		}
		root >>= 1
		place >>= 2
	}
	return root
}

// Isqrt32 computes the integer square root of a uint32.
func Isqrt32(n uint32) uint32 {
	var root, remainder, place uint32
	remainder = n
	place = 0x40000000
	for place > remainder {
		place >>= 2
	}
	for place != 0 {
		if remainder >= root+place {
			remainder -= root + place
			root += place << 1
		}
		root >>= 1
		place >>= 2
	}
	return root
}
