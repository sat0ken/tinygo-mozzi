// Package rollingaverage is a port of Mozzi's RollingAverage.h: a moving
// average of the last few inputs, typically for smoothing a noisy control
// signal such as an analog sensor reading.
package rollingaverage

// Integer is the set of types RollingAverage works with.
type Integer interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}

// RollingAverage averages the last WindowLength inputs, ported from
// upstream's integer RollingAverage<T, WINDOW_LENGTH> (and its unsigned int
// specialization, which this covers too): the average is a running total
// shifted right by log2(window length).
type RollingAverage[T Integer] struct {
	readings []T
	index    int
	total    int64
	shift    uint8
}

// New creates a RollingAverage over the last windowLength inputs, which must
// be a power of two (upstream's WINDOW_LENGTH template parameter).
func New[T Integer](windowLength int) *RollingAverage[T] {
	if windowLength < 1 || windowLength&(windowLength-1) != 0 {
		panic("rollingaverage: window length must be a power of two")
	}
	r := &RollingAverage[T]{readings: make([]T, windowLength)}
	for n := windowLength; n > 1; n >>= 1 {
		r.shift++
	}
	return r
}

// Next adds input to the window and returns the average of the window.
func (r *RollingAverage[T]) Next(input T) T {
	r.total += int64(input) - int64(r.readings[r.index])
	r.readings[r.index] = input
	r.index = (r.index + 1) & (len(r.readings) - 1)
	return T(r.total >> r.shift)
}

// Float is upstream's RollingAverage<float, WINDOW_LENGTH> specialization.
type Float struct {
	readings []float32
	index    int
	total    float32
}

// NewFloat creates a Float RollingAverage over the last windowLength inputs
// (a power of two).
func NewFloat(windowLength int) *Float {
	if windowLength < 1 || windowLength&(windowLength-1) != 0 {
		panic("rollingaverage: window length must be a power of two")
	}
	return &Float{readings: make([]float32, windowLength)}
}

// Next adds input to the window and returns the average of the window.
func (r *Float) Next(input float32) float32 {
	r.total += input - r.readings[r.index]
	r.readings[r.index] = input
	r.index = (r.index + 1) & (len(r.readings) - 1)
	return r.total / float32(len(r.readings))
}
