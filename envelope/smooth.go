package envelope

import "mozzi/fixmath"

// SmoothInt32 is a simple one-pole IIR low-pass filter for smoothing control
// or audio signals of general integer range, ported from the primary
// (non-specialized) template in Mozzi's Smooth.h -- i.e. what you get from
// upstream's `Smooth<int>` or `Smooth<long>`.
//
// out = last_out + a*(in - last_out)
//
// It isn't calibrated to any particular update rate: if you use it at your
// control rate and then change the control rate, re-tune Smoothness.
type SmoothInt32 struct {
	lastOut int64
	a       fixmath.Q0n16
}

// NewSmoothInt32 creates a smoother. smoothness is in [0,1); 0 barely
// smooths, close to 1 smooths heavily.
func NewSmoothInt32(smoothness float32) *SmoothInt32 {
	s := &SmoothInt32{}
	s.SetSmoothness(smoothness)
	return s
}

// Next filters one input sample and returns the smoothed result.
func (s *SmoothInt32) Next(in int32) int32 {
	out := (((int64(in) - (s.lastOut >> 8)) * int64(s.a)) >> 8) + s.lastOut
	s.lastOut = out
	return int32(out >> 8)
}

// SetSmoothness sets how much smoothing is applied, in [0,1).
func (s *SmoothInt32) SetSmoothness(smoothness float32) {
	s.a = fixmath.FloatToQ0n16(1 - smoothness)
}

// SmoothUint8 is the uint8 specialization of Smooth: cheaper than
// SmoothInt32, at the cost of resolution.
type SmoothUint8 struct {
	lastOut uint32
	a       fixmath.Q0n8
}

func NewSmoothUint8(smoothness float32) *SmoothUint8 {
	s := &SmoothUint8{}
	s.SetSmoothness(smoothness)
	return s
}

func (s *SmoothUint8) Next(in uint8) uint8 {
	out := uint32(int32(in)-int32(s.lastOut>>8))*uint32(s.a) + s.lastOut
	s.lastOut = out
	return uint8(out >> 8)
}

func (s *SmoothUint8) SetSmoothness(smoothness float32) {
	s.a = fixmath.FloatToQ0n8(1 - smoothness)
}

// SmoothInt8 is the int8 specialization of Smooth.
type SmoothInt8 struct {
	lastOut int32
	a       fixmath.Q0n8
}

func NewSmoothInt8(smoothness float32) *SmoothInt8 {
	s := &SmoothInt8{}
	s.SetSmoothness(smoothness)
	return s
}

func (s *SmoothInt8) Next(in int8) int8 {
	out := (int32(in)-(s.lastOut>>8))*int32(s.a) + s.lastOut
	s.lastOut = out
	return int8(out >> 8)
}

func (s *SmoothInt8) SetSmoothness(smoothness float32) {
	s.a = fixmath.FloatToQ0n8(1 - smoothness)
}

// SmoothFloat32 is the float32 specialization of Smooth.
type SmoothFloat32 struct {
	lastOut float32
	a       float32
}

func NewSmoothFloat32(smoothness float32) *SmoothFloat32 {
	s := &SmoothFloat32{}
	s.SetSmoothness(smoothness)
	return s
}

func (s *SmoothFloat32) Next(in float32) float32 {
	out := s.lastOut + s.a*(in-s.lastOut)
	s.lastOut = out
	return out
}

func (s *SmoothFloat32) SetSmoothness(smoothness float32) { s.a = 1 - smoothness }
