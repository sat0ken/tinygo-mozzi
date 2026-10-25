// Package midi is a port of Mozzi's mozzi_midi.h: MIDI note number to
// frequency conversions.
package midi

import (
	"math"

	"mozzi/fixmath"
)

// midiToFreq is Q16n16 frequency values for MIDI notes 0..127, ported
// verbatim from MidiToFreqPrivate::midiToFreq in mozzi_midi.h. Note 0 is a
// placeholder (0Hz); see MtoF's note about it being invalid, same as upstream.
var midiToFreq = [128]fixmath.Q16n16{
	0, 567670, 601425, 637188, 675077, 715219, 757748, 802806, 850544, 901120,
	954703, 1011473, 1071618, 1135340, 1202851, 1274376, 1350154, 1430438, 1515497,
	1605613, 1701088, 1802240, 1909406, 2022946, 2143236, 2270680, 2405702, 2548752,
	2700309, 2860877, 3030994, 3211226, 3402176, 3604479, 3818813, 4045892, 4286472,
	4541359, 4811404, 5097504, 5400618, 5721756, 6061988, 6422452, 6804352, 7208959,
	7637627, 8091785, 8572945, 9082719, 9622808, 10195009, 10801235, 11443507,
	12123974, 12844905, 13608704, 14417917, 15275252, 16183563, 17145888, 18165438,
	19245616, 20390018, 21602470, 22887014, 24247948, 25689810, 27217408, 28835834,
	30550514, 32367136, 34291776, 36330876, 38491212, 40780036, 43204940, 45774028,
	48495912, 51379620, 54434816, 57671668, 61101028, 64734272, 68583552, 72661752,
	76982424, 81560072, 86409880, 91548056, 96991792, 102759240, 108869632,
	115343336, 122202056, 129468544, 137167104, 145323504, 153964848, 163120144,
	172819760, 183096224, 193983648, 205518336, 217739200, 230686576, 244403840,
	258937008, 274334112, 290647008, 307929696, 326240288, 345639520, 366192448,
	387967040, 411036672, 435478400, 461373152, 488807680, 517874016, 548668224,
	581294016, 615859392, 652480576, 691279040, 732384896, 775934592, 822073344,
}

// MtoF converts a (possibly fractional) MIDI note number to frequency in Hz.
// Caution: this can take a while to run (it calls math.Pow), so avoid it
// inside updateControl()/updateAudio() -- call it in setup, or wherever
// precise conversion matters more than speed. Note 0 returns 0, same as upstream.
func MtoF(midival float32) float32 {
	if midival == 0 {
		return 0
	}
	return 8.1757989156 * float32(math.Pow(2.0, float64(midival)/12.0))
}

// MtoFWhole is a fast, low-accuracy lookup-table conversion for whole MIDI
// note numbers (0..127).
func MtoFWhole(midiNote uint8) int {
	return int(midiToFreq[midiNote] >> 16)
}

// Q16n16MtoF converts a MIDI note number in Q16n16 format to frequency, also
// in Q16n16 format, interpolating linearly between the whole-note lookup
// table entries. Fast and reasonably accurate; a good default for setFreq.
func Q16n16MtoF(midivalFractional fixmath.Q16n16) fixmath.Q16n16 {
	index := uint8(midivalFractional >> 16)
	fraction := uint16(midivalFractional) // low word
	// Upstream reads one past the 128-entry table when index==127 (harmless,
	// if inaccurate, on a flat address space). Clamp the *second* lookup
	// instead, to avoid a Go slice-bounds panic while still returning
	// exactly midiToFreq[127] for the (common, fraction==0) whole-note case.
	if index > 127 {
		index = 127
	}
	index2 := index
	if index2 < 127 {
		index2++
	}
	freq1 := midiToFreq[index]
	freq2 := midiToFreq[index2]
	difference := freq2 - freq1

	var diffFraction fixmath.Q16n16
	if difference >= 65536 {
		diffFraction = ((difference >> 8) * uint32(fraction)) >> 8
	} else {
		diffFraction = (difference * uint32(fraction)) >> 16
	}
	return freq1 + diffFraction
}
