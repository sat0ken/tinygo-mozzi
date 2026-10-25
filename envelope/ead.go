package envelope

import (
	"math"

	"mozzi/fixmath"
)

// Ead is an exponential attack-decay envelope, producing a more natural
// sounding envelope than ADSR's linear ramps. Ported from Mozzi's Ead.h
// (itself adapted from the ead~ Pure Data external).
type Ead struct {
	updateRate uint32

	q8n8Attack  fixmath.Q8n8
	q8n8Decay   fixmath.Q8n8
	q8n24State  fixmath.Q8n24
	attackPhase bool
}

// NewEad creates an Ead updated at updateRate, usually your engine's control
// or audio rate.
func NewEad(updateRate uint32) *Ead {
	return &Ead{updateRate: updateRate}
}

// SetAttack sets the attack time in milliseconds: how long it takes Next()
// to rise from 0 to 255.
func (e *Ead) SetAttack(attackMs uint32) {
	e.q8n8Attack = fixmath.FloatToQ8n8(e.millisToOneMinusRealPole(attackMs))
}

// SetDecay sets the decay time in milliseconds: how long it takes Next() to
// fall from 255 to 0.
func (e *Ead) SetDecay(decayMs uint32) {
	e.q8n8Decay = fixmath.FloatToQ8n8(e.millisToOneMinusRealPole(decayMs))
}

// Set sets both attack and decay times, in milliseconds.
func (e *Ead) Set(attackMs, decayMs uint32) {
	e.SetAttack(attackMs)
	e.SetDecay(decayMs)
}

// Start (re)starts the envelope from the beginning; can be called at any
// time, even mid-envelope.
func (e *Ead) Start() {
	e.q8n24State = 0
	e.attackPhase = true
}

// StartWith sets the attack/decay times and starts the envelope, without
// resetting the current level (avoids a click if re-triggered mid-envelope).
func (e *Ead) StartWith(attackMs, decayMs uint32) {
	e.Set(attackMs, decayMs)
	e.attackPhase = true
}

// Next computes and returns the next envelope value, in the range 0-255.
func (e *Ead) Next() uint8 {
	if e.attackPhase {
		// multiply A(a1,b1) * A(a2,b2) = A(a1+a2, b1+b2)
		e.q8n24State += ((fixmath.Q8n24(fixmath.Q8n24Fix1) - e.q8n24State) * fixmath.Q8n24(e.q8n8Attack)) >> 8
		if e.q8n24State >= fixmath.Q8n24Fix1-256 {
			e.q8n24State = fixmath.Q8n24Fix1 - 256
			e.attackPhase = false
		}
	} else {
		e.q8n24State -= (e.q8n24State * fixmath.Q8n24(e.q8n8Decay)) >> 8
	}
	return fixmath.Q8n24ToQ0n8(e.q8n24State)
}

// millisToOneMinusRealPole converts milliseconds to 1-p, with p a real pole.
func (e *Ead) millisToOneMinusRealPole(milliseconds uint32) float32 {
	const numerator = 1000.0 * -6.907755 // 1000 * ln(0.001), precomputed to avoid pulling in float64 math.Log on every call
	return -expm1(numerator / (float32(e.updateRate) * float32(milliseconds)))
}

// expm1 computes exp(x)-1 without loss of precision for small x.
func expm1(x float32) float32 {
	abs := x
	if abs < 0 {
		abs = -abs
	}
	if abs < 1e-5 {
		return x + 0.5*x*x
	}
	return float32(math.Exp(float64(x))) - 1
}
