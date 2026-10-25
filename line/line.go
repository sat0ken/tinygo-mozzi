// Package line is a port of Mozzi's Line.h: linear interpolation/ramping
// with a minimum of work per step (one addition).
//
// Upstream ships template specializations for unsigned char, unsigned int
// and unsigned long alongside the generic case, so that an unsigned Line
// can still ramp *down*: they keep step_size in a signed type and compute it
// as a float, ((float)target - current) / num_steps. Without that, the
// generic (target - current) / num_steps wraps around for an unsigned T
// whenever target < current, producing a huge bogus step. This port keeps a
// single generic Line[T] and handles that case in SetTarget instead (see
// there), staying in integer arithmetic rather than going through float.
package line

// Number is the set of types Line can ramp between.
type Number interface {
	~int8 | ~int16 | ~int32 | ~int64 |
		~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Line produces linear changes with a minimum of calculation at each step.
// For instance, you can make an oscillator glide from one frequency to
// another by precalculating the phase increments for each end and letting a
// Line change the phase increment with a simple addition at each step.
//
// Watch out for underflow in the internal step-size calculation if you're
// not using floats: if a Line seems to do nothing, its step size is likely
// truncating to zero. Scale up the values you're using (mozzi/fixmath's
// fixed-point types are meant for exactly this).
type Line[T Number] struct {
	currentValue T
	stepSize     T
}

// New creates a Line with a zero current value and step size.
func New[T Number]() *Line[T] { return &Line[T]{} }

// Next increments one step along the line and returns the new value.
func (l *Line[T]) Next() T {
	l.currentValue += l.stepSize
	return l.currentValue
}

// Set sets the current value. The Line continues incrementing from here
// using whatever step size was last calculated.
func (l *Line[T]) Set(value T) { l.currentValue = value }

// SetTarget computes the step size needed to reach targetValue from the
// current value in numSteps steps.
func (l *Line[T]) SetTarget(targetValue T, numSteps T) {
	if numSteps == 0 {
		l.stepSize = 0
		l.currentValue = targetValue
		return
	}
	var zero T
	if zero-1 > zero && targetValue < l.currentValue {
		// Unsigned T ramping down: target-current would wrap, so divide the
		// (positive) distance instead and negate it modulo 2^n. Next()'s
		// addition wraps the same way, so adding this step subtracts the
		// distance/numSteps -- what upstream's signed step_size achieves.
		l.stepSize = zero - (l.currentValue-targetValue)/numSteps
		return
	}
	l.stepSize = (targetValue - l.currentValue) / numSteps
}

// SetFromTo sets a new starting value, then computes the step size needed
// to reach targetValue in numSteps steps.
func (l *Line[T]) SetFromTo(startValue, targetValue, numSteps T) {
	l.Set(startValue)
	l.SetTarget(targetValue, numSteps)
}
