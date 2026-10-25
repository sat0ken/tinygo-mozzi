package tables

import "unsafe"

// These helpers reinterpret an embedded, flash-resident byte blob as the
// wavetable's real element type, without copying it into RAM. That's safe
// here because every generated blob's length is a multiple of the element
// size, and single-byte element types have no alignment requirement.
//
// asInt16LE/asUint16LE do allocate a small RAM-resident copy: an
// arbitrary //go:embed byte slice is only guaranteed byte alignment, which
// is not sufficient to reinterpret as []int16 on all TinyGo targets. Only
// two upstream tables use 16-bit cells, so this costs a few KB at most.

func asInt8(b []byte) []int8 {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*int8)(unsafe.Pointer(unsafe.SliceData(b))), len(b))
}

func asUint8(b []byte) []uint8 {
	return b
}

func asInt16LE(b []byte) []int16 {
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(uint16(b[2*i]) | uint16(b[2*i+1])<<8)
	}
	return out
}

func asUint16LE(b []byte) []uint16 {
	out := make([]uint16, len(b)/2)
	for i := range out {
		out[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return out
}
