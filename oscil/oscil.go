// Package oscil is a port of Mozzi's Oscil.h: a wavetable oscillator that
// cycles through a table to generate an audio or control signal.
//
// Upstream makes NUM_TABLE_CELLS and UPDATE_RATE template parameters so the
// phase-increment shift math can be constant-folded at compile time. Go has
// no non-generic compile-time constant templates of that kind, so this port
// keeps the table length and update rate as ordinary fields, computed once
// in New()/SetTable() and SetFreq(), rather than re-derived per sample. The
// RP2040 this targets is easily fast enough for the extra multiply/divide
// this costs versus the AVR original.
package oscil

import "mozzi/fixmath"

const (
	fBits           = 16    // fractional bits for oscillator index precision
	fBitsMultiplier = 65536 // 1 << fBits
)

// Oscil plays an int8 wavetable, cycling through it to generate an audio or
// control signal. Frequency is set with one of the SetFreq* methods, and
// output is produced with Next() (or PhMod() for phase modulation).
type Oscil struct {
	table                    []int8
	tableMask                uint32 // len(table)-1; table length must be a power of two
	updateRate               uint32 // MOZZI_AUDIO_RATE if driven from UpdateAudio, MOZZI_CONTROL_RATE if from UpdateControl
	phaseFractional          uint32
	phaseIncrementFractional uint32
}

// New creates an Oscil that reads the given table, updated at updateRate
// (typically your engine's audio rate for an audio-rate oscillator, or its
// control rate for a control-rate one). len(table) must be a power of two.
func New(table []int8, updateRate uint32) *Oscil {
	o := &Oscil{updateRate: updateRate}
	o.SetTable(table)
	return o
}

// SetTable changes the wavetable played by the Oscil. len(table) must be a
// power of two.
func (o *Oscil) SetTable(table []int8) {
	o.table = table
	o.tableMask = uint32(len(table)) - 1
}

// Next advances the phase according to the current frequency and returns
// the sample at the new phase position.
func (o *Oscil) Next() int8 {
	o.incrementPhase()
	return o.readTable()
}

// SetPhase sets the phase of the Oscil to a position in the wavetable.
func (o *Oscil) SetPhase(phase uint32) { o.phaseFractional = phase << fBits }

// SetPhaseFractional sets the phase directly in fractional (phase<<fBits) format.
func (o *Oscil) SetPhaseFractional(phase uint32) { o.phaseFractional = phase }

// PhaseFractional returns the current phase, left-shifted by the internal
// fractional bit count (16).
func (o *Oscil) PhaseFractional() uint32 { return o.phaseFractional }

// PhMod returns the next sample given a phase modulation value, expressed as
// a Q15n16 fixed-point proportion of the whole table length (roughly -1..1).
func (o *Oscil) PhMod(phmodProportion fixmath.Q15n16) int8 {
	o.incrementPhase()
	idx := (o.phaseFractional + uint32(int32(phmodProportion)*int32(len(o.table)))) >> fBits
	return o.table[idx&o.tableMask]
}

// SetFreq sets the oscillator frequency from an int. Fast, but can be
// imprecise for low/high frequencies depending on table size -- try
// SetFreqFloat, SetFreqQ24n8 or SetFreqQ16n16 if results look off.
func (o *Oscil) SetFreq(frequency int) {
	o.phaseIncrementFractional = uint32(frequency) * ((fBitsMultiplier * uint32(len(o.table))) / o.updateRate)
}

// SetFreqFloat sets the oscillator frequency from a float32. The most
// reliable way to set fractional frequencies.
func (o *Oscil) SetFreqFloat(frequency float32) {
	o.phaseIncrementFractional = uint32((float32(len(o.table)) * frequency / float32(o.updateRate)) * fBitsMultiplier)
}

// SetFreqQ24n8 sets the frequency in Q24n8 fixed-point format. Can be more
// reliable than SetFreq for low frequencies. Update rate must be >= 64Hz.
func (o *Oscil) SetFreqQ24n8(frequency fixmath.Q24n8) {
	n := uint32(len(o.table))
	if 256*n >= o.updateRate {
		o.phaseIncrementFractional = frequency * ((256 * n) / o.updateRate)
	} else {
		o.phaseIncrementFractional = frequency / (o.updateRate / (256 * n))
	}
}

// SetFreqQ16n16 sets the frequency in Q16n16 fixed-point format, useful with
// midi.Q16n16MtoF. Table should be <=2048 cells and frequency <=4096Hz for
// good results; update rate must be >= 64Hz.
func (o *Oscil) SetFreqQ16n16(frequency fixmath.Q16n16) {
	n := uint32(len(o.table))
	if n >= o.updateRate {
		o.phaseIncrementFractional = frequency * (n / o.updateRate)
	} else {
		o.phaseIncrementFractional = frequency / (o.updateRate / n)
	}
}

// AtIndex returns the sample at the given table index (wrapping around if
// index is larger than the table size).
func (o *Oscil) AtIndex(index uint32) int8 {
	return o.table[index&o.tableMask]
}

// PhaseIncFromFreq returns the phase increment value that would produce the
// given frequency, for use with SetPhaseInc -- handy for sliding smoothly
// between two frequencies with a mozzi/line.Line instead of recomputing
// SetFreq() every step.
func (o *Oscil) PhaseIncFromFreq(frequency int) uint32 {
	return uint32(frequency) * ((fBitsMultiplier * uint32(len(o.table))) / o.updateRate)
}

// SetPhaseInc sets a specific phase increment, see PhaseIncFromFreq.
func (o *Oscil) SetPhaseInc(phaseIncFractional uint32) {
	o.phaseIncrementFractional = phaseIncFractional
}

func (o *Oscil) incrementPhase() { o.phaseFractional += o.phaseIncrementFractional }

func (o *Oscil) readTable() int8 {
	return o.table[(o.phaseFractional>>fBits)&o.tableMask]
}
