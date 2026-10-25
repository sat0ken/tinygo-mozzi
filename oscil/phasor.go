package oscil

// phasorMaxValue is the maximum value a uint32 phase can hold before wrapping.
const phasorMaxValue = 4294967295

// Phasor repeatedly generates a high resolution ramp at a variable
// frequency. Next() returns a value between 0 and 4294967295 (the full
// range of a uint32), ported from Mozzi's Phasor.h.
type Phasor struct {
	currentValue uint32
	stepSize     uint32
	updateRate   uint32
}

// NewPhasor creates a Phasor updated at updateRate (usually your engine's
// control or audio rate).
func NewPhasor(updateRate uint32) *Phasor {
	return &Phasor{updateRate: updateRate}
}

// Next increments one step along the phase (wrapping) and returns it.
func (p *Phasor) Next() uint32 {
	p.currentValue += p.stepSize
	return p.currentValue
}

// Set sets the current value of the phasor; it continues incrementing from
// here using the previously set step size.
func (p *Phasor) Set(value uint32) { p.currentValue = value }

// SetFreq sets how many times per second the Phasor counts from 0 to
// 4294967295.
func (p *Phasor) SetFreq(frequency int) {
	p.stepSize = (((phasorMaxValue >> 8) + 1) / p.updateRate * uint32(frequency)) << 8
}

// SetFreqFloat sets the Phasor frequency from a float32.
func (p *Phasor) SetFreqFloat(frequency float32) {
	p.stepSize = uint32(float32(phasorMaxValue) / float32(p.updateRate) * frequency)
}

// PhaseIncFromFreq returns the phase increment that would produce the given
// frequency, for use with SetPhaseInc.
func (p *Phasor) PhaseIncFromFreq(frequency int) uint32 {
	return (((phasorMaxValue >> 8) + 1) / p.updateRate * uint32(frequency)) << 8
}

// SetPhaseInc sets a specific phase increment, see PhaseIncFromFreq.
func (p *Phasor) SetPhaseInc(stepSize uint32) { p.stepSize = stepSize }
