// Package delay is a port of Mozzi's AudioDelay.h and (a subset of)
// AudioDelayFeedback.h: simple ring-buffer delay lines for comb filter,
// flange, chorus and echo effects.
package delay

// Cell is the set of element types an AudioDelay line can hold.
type Cell interface {
	~int8 | ~int16 | ~int32 | ~int64
}

// AudioDelay is a delay line without feedback (for feedback, see
// AudioDelayFeedback), ported from Mozzi's AudioDelay.h.
type AudioDelay[T Cell] struct {
	buf            []T
	mask           uint32
	writePos       uint32
	delayTimeCells uint32
}

// New creates an AudioDelay with a buffer of numBufferSamples cells (must be
// a power of two). The largest delay this can hold is
// numBufferSamples/yourAudioRate seconds.
func New[T Cell](numBufferSamples int) *AudioDelay[T] {
	return &AudioDelay[T]{buf: make([]T, numBufferSamples), mask: uint32(numBufferSamples) - 1}
}

// NewWithDelay is like New, additionally setting the initial delay time.
func NewWithDelay[T Cell](numBufferSamples int, delayTimeCells uint32) *AudioDelay[T] {
	d := New[T](numBufferSamples)
	d.delayTimeCells = delayTimeCells
	return d
}

// Set sets the delay time, measured in cells (samples).
func (d *AudioDelay[T]) Set(delayTimeCells uint32) { d.delayTimeCells = delayTimeCells }

// Next writes inValue into the delay and returns the signal that was
// delayTimeCells (as set by Set) behind it.
func (d *AudioDelay[T]) Next(inValue T) T {
	return d.NextWithDelay(inValue, d.delayTimeCells)
}

// NextWithDelay is like Next, but takes the delay time (in cells) directly,
// without changing the stored delay time.
func (d *AudioDelay[T]) NextWithDelay(inValue T, delayTimeCells uint32) T {
	d.writePos = (d.writePos + 1) & d.mask
	readPos := (d.writePos - delayTimeCells) & d.mask
	d.buf[d.writePos] = inValue
	return d.buf[readPos]
}

// Read retrieves the signal at delayTimeCells behind the write head, without
// writing or advancing anything.
func (d *AudioDelay[T]) Read(delayTimeCells uint32) T {
	readPos := (d.writePos - delayTimeCells) & d.mask
	return d.buf[readPos]
}
