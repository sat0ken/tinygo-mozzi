// Package filter is a port of Mozzi's ResonantFilter.h: a simple resonant
// filter (Paul Kellett's design, as adapted by Peter Schoffhauzer to expose
// all four of lowpass/highpass/bandpass/notch from shared state), originally
// posted to musicdsp.org:
//
//	fb = q + q/(1.0 - f)   (approximated via a Taylor expansion, see below)
//	buf0 += f * (in - buf0 + fb*(buf0 - buf1))
//	buf1 += f * (buf0 - buf1)
//	low    = buf1
//	high   = in - buf0
//	band   = buf0 - buf1
//	notch  = in - buf0 + buf1
//
// Upstream exposes this as two families: ResonantFilter<FILTER_TYPE, su>
// (pick one output at construction time) and MultiResonantFilter<su> (all
// four outputs from one filter), and thanks to C++ template metaprogramming
// (IntegerType<>), each intermediate product upstream uses the narrowest
// integer width it can get away with. This port always computes all four
// outputs -- one extra subtraction each, negligible on a 32 bit MCU -- and
// keeps every product in a single, consistently wide type (int64 for
// products, int32 for state) rather than chasing upstream's exact
// per-subexpression widths. That reproduces the documented algorithm
// faithfully for the whole of its normal (non-overflowing) operating range,
// without trying to bit-replicate incidental truncation from a
// template-selected width that has no algorithmic meaning of its own.
//
// Filter8 takes 8 bit (0-255) cutoff/resonance, matching upstream's
// LowPassFilter/ResonantFilter<...,uint8_t>. Filter16 takes 16 bit
// (0-65535) cutoff/resonance, matching ResonantFilter16.
package filter

// Filter8 is a resonant filter with 8 bit cutoff/resonance control,
// operating on audioout.Storage-range (int32) samples.
type Filter8 struct {
	q, f       int32 // 0-255
	fb         int32
	buf0, buf1 int32
	lastIn     int32
}

// NewFilter8 creates a Filter8 with cutoff and resonance both at 0.
func NewFilter8() *Filter8 { return &Filter8{} }

// SetCutoffFreqAndResonance sets the cutoff (0-255, representing 0 to half
// your audio rate) and resonance (0-255, 255 being most resonant). Be
// careful of distortion at low cutoff with high resonance.
func (f *Filter8) SetCutoffFreqAndResonance(cutoff, resonance uint8) {
	f.f = int32(cutoff)
	f.q = int32(resonance)
	f.fb = f.q + ((f.q * ((255 + f.f) >> 1)) >> 7)
}

// Next advances the filter with one input sample. Call one of the
// accessors (Low/High/Band/Notch) afterwards to read the corresponding output.
func (f *Filter8) Next(in int32) {
	f.buf0 += int32((int64(in-f.buf0) + (int64(f.fb)*int64(f.buf0-f.buf1))>>8) * int64(f.f) >> 8)
	f.buf1 += int32(int64(f.buf0-f.buf1) * int64(f.f) >> 8)
	f.lastIn = in
}

func (f *Filter8) Low() int32   { return f.buf1 }
func (f *Filter8) High() int32  { return f.lastIn - f.buf0 }
func (f *Filter8) Band() int32  { return f.buf0 - f.buf1 }
func (f *Filter8) Notch() int32 { return f.lastIn - f.buf0 + f.buf1 }

// Filter16 is a resonant filter with 16 bit cutoff/resonance control.
type Filter16 struct {
	q, f       int32 // 0-65535
	fb         int32
	buf0, buf1 int32
	lastIn     int32
}

func NewFilter16() *Filter16 { return &Filter16{} }

// SetCutoffFreqAndResonance sets the cutoff (0-65535) and resonance
// (0-65535, 65535 being most resonant).
func (f *Filter16) SetCutoffFreqAndResonance(cutoff, resonance uint16) {
	f.f = int32(cutoff)
	f.q = int32(resonance)
	f.fb = f.q + int32((int64(f.q)*int64((65535+f.f)>>1))>>15)
}

func (f *Filter16) Next(in int32) {
	f.buf0 += int32((int64(in-f.buf0) + (int64(f.fb)*int64(f.buf0-f.buf1))>>16) * int64(f.f) >> 16)
	f.buf1 += int32(int64(f.buf0-f.buf1) * int64(f.f) >> 16)
	f.lastIn = in
}

func (f *Filter16) Low() int32   { return f.buf1 }
func (f *Filter16) High() int32  { return f.lastIn - f.buf0 }
func (f *Filter16) Band() int32  { return f.buf0 - f.buf1 }
func (f *Filter16) Notch() int32 { return f.lastIn - f.buf0 + f.buf1 }
