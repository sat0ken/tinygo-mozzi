package delay

// AudioDelayFeedback is a delay line with feedback, for comb filter, flange,
// chorus and reverb-like effects. Ported from Mozzi's AudioDelayFeedback.h,
// int8 cells, LINEAR interpolation only.
//
// Not ported (left as a documented gap; see README): the ALLPASS
// interpolation mode, and the int16-cell template variant. Both are
// reasonably mechanical extensions of this file, if you need them.
type AudioDelayFeedback struct {
	buf            []int32 // wider than upstream's int16 "return_type", to avoid incidental overflow
	mask           uint32
	writePos       uint32
	feedbackLevel  int32 // -128..127, representing -1 to ~1
	delayTimeCells uint32
}

// NewAudioDelayFeedback creates a delay line with a buffer of
// numBufferSamples cells (must be a power of two).
func NewAudioDelayFeedback(numBufferSamples int) *AudioDelayFeedback {
	return &AudioDelayFeedback{buf: make([]int32, numBufferSamples), mask: uint32(numBufferSamples) - 1}
}

// SetDelayTimeCells sets the delay time, in cells (samples).
func (d *AudioDelayFeedback) SetDelayTimeCells(cells uint32) { d.delayTimeCells = cells }

// SetFeedbackLevel sets the feedback gain, from -128 to 127 (representing -1 to ~1).
func (d *AudioDelayFeedback) SetFeedbackLevel(level int8) { d.feedbackLevel = int32(level) }

// Next writes input into the delay (plus feedback from the output) and
// returns the delay line's signal at the stored delay time.
func (d *AudioDelayFeedback) Next(input int8) int32 {
	return d.NextWithDelay(input, d.delayTimeCells)
}

// NextWithDelay is like Next, but takes the delay time directly, without
// changing the stored delay time.
func (d *AudioDelayFeedback) NextWithDelay(input int8, delayTimeCells uint32) int32 {
	d.writePos = (d.writePos + 1) & d.mask
	readPos := (d.writePos - delayTimeCells) & d.mask
	delaySig := d.buf[readPos]
	feedbackSig := clipInt8((delaySig * d.feedbackLevel) >> 7)
	d.buf[d.writePos] = int32(input) + feedbackSig
	return delaySig
}

// Write feeds a value into the delay without reading or applying feedback.
func (d *AudioDelayFeedback) Write(input int8) {
	d.writePos = (d.writePos + 1) & d.mask
	d.buf[d.writePos] = int32(input)
}

// WriteFeedback writes at the current write position without advancing it;
// useful for manually adding feedback "behind" the advancing write head.
func (d *AudioDelayFeedback) WriteFeedback(input int8) {
	d.buf[d.writePos] = int32(input)
}

func clipInt8(v int32) int32 {
	if v < -128 {
		return -128
	}
	if v > 127 {
		return 127
	}
	return v
}
