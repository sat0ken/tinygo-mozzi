// Package audioout is a port of the sample-frame API from Mozzi's
// AudioOutput.h: a small wrapper around one frame of audio, letting
// updateAudio() implementations declare how many bits wide their samples
// are so the backend can scale them portably.
//
// Simplification vs. upstream: upstream's output resolution
// (MOZZI_AUDIO_BITS) is a compile-time constant that varies per platform
// and output mode (8 bits on classic AVR PWM, 14 on AVR HIFI, 10-12 on many
// DAC/PWM setups, etc). This port fixes a single logical resolution,
// Bits, for every backend, and lets each hardware backend (see
// mozzi/backend/...) rescale from that logical resolution to whatever its
// DAC/PWM actually supports. In practice this means: build your
// updateAudio() the same way you would for classic 8 bit Mozzi
// (From8Bit/FromNBit(8, ...)), and it will sound right on any backend this
// port ships.
package audioout

// Bits is the logical output resolution assumed throughout this port. See
// the package doc for why this differs from upstream's MOZZI_AUDIO_BITS.
const Bits = 8

// Bias is half the full-scale range at the logical resolution, i.e. the
// magnitude of the most negative sample.
const Bias = 1 << (Bits - 1)

// Storage is the type used to store one channel of one audio frame,
// internally. Kept wide (int32) since intermediate sums (e.g. mixing a few
// oscillators) can exceed the 8 bit logical range before Clip()ping.
type Storage = int32

// scaleAudio mirrors upstream's SCALE_AUDIO macro: shift a value known to be
// in an `bits`-bit range so that it fills the logical Bits-bit range.
func scaleAudio(x Storage, bits uint8) Storage {
	if bits > Bits {
		return x >> (bits - Bits)
	}
	return x << (Bits - bits)
}

// clip constrains x to the representable logical range, same as upstream's
// CLIP_AUDIO.
func clip(x Storage) Storage {
	if x < -Bias {
		return -Bias
	}
	if x > Bias-1 {
		return Bias - 1
	}
	return x
}

// Mono is one frame of mono audio output, ported from upstream's MonoOutput.
type Mono struct {
	l Storage
}

// FromNBit constructs a frame from a zero-centered value known to be in the
// `bits`-bit range, scaling it (left- or right-shifting) to the logical
// resolution.
func FromNBit(bits uint8, l Storage) Mono { return Mono{l: scaleAudio(l, bits)} }

// FromAlmostNBit is upstream's fromAlmostNBit: for a value that's "almost
// but not quite" in the bits-bit range. Upstream only treats it differently
// from FromNBit on AVR's 8-and-a-half-bit PWM mode (MOZZI_AUDIO_BITS_OPTIMISTIC);
// everywhere else, as here, it's the same as FromNBit. Follow it with Clip()
// if a few stray values may exceed the range.
func FromAlmostNBit(bits uint8, l Storage) Mono { return FromNBit(bits, l) }

// From8Bit constructs a frame from a value in the 8 bit range ([-128,127]).
// This is what most simple updateAudio() functions (a single Oscil.Next())
// should use.
func From8Bit(l int16) Mono { return FromNBit(8, Storage(l)) }

// From16Bit constructs a frame from a value in the 16 bit range.
func From16Bit(l int32) Mono { return FromNBit(16, Storage(l)) }

// L returns the (only) channel of the frame.
func (m Mono) L() Storage { return m.l }

// R returns the same value as L(), for source compatibility with Stereo.
func (m Mono) R() Storage { return m.l }

// Clip constrains the frame to the representable range. Useful when mixing
// several signals that occasionally exceeds the usual range.
func (m Mono) Clip() Mono { return Mono{l: clip(m.l)} }

// Stereo is one frame of stereo audio output, ported from upstream's StereoOutput.
type Stereo struct {
	l, r Storage
}

// NewStereo constructs a raw, zero-centered stereo frame at the logical resolution.
func NewStereo(l, r Storage) Stereo { return Stereo{l: l, r: r} }

// StereoFromNBit constructs a stereo frame from values in the `bits`-bit range.
func StereoFromNBit(bits uint8, l, r Storage) Stereo {
	return Stereo{l: scaleAudio(l, bits), r: scaleAudio(r, bits)}
}

// StereoFromAlmostNBit is the stereo variant of FromAlmostNBit.
func StereoFromAlmostNBit(bits uint8, l, r Storage) Stereo { return StereoFromNBit(bits, l, r) }

// StereoFrom8Bit constructs a stereo frame from values in the 8 bit range.
func StereoFrom8Bit(l, r int16) Stereo { return StereoFromNBit(8, Storage(l), Storage(r)) }

// StereoFrom16Bit constructs a stereo frame from values in the 16 bit range.
func StereoFrom16Bit(l, r int32) Stereo { return StereoFromNBit(16, Storage(l), Storage(r)) }

// L returns the left channel.
func (s Stereo) L() Storage { return s.l }

// R returns the right channel.
func (s Stereo) R() Storage { return s.r }

// Clip constrains both channels to the representable range.
func (s Stereo) Clip() Stereo { return Stereo{l: clip(s.l), r: clip(s.r)} }
