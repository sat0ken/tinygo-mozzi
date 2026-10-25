// Package envelope is a port of Mozzi's ADSR.h, Ead.h and Smooth.h.
package envelope

import (
	"mozzi/fixmath"
	"mozzi/line"
)

type adsrPhaseKind uint8

const (
	phaseAttack adsrPhaseKind = iota
	phaseDecay
	phaseSustain
	phaseRelease
	phaseIdle
)

type adsrPhase struct {
	kind        adsrPhaseKind
	updateSteps uint32
	lerpSteps   int32
	level       uint8
}

// ADSR is a simple attack/decay/sustain/release envelope generator. Update()
// (called at control rate) advances through the phases; Next() (called
// wherever you need the value, typically at audio rate) interpolates
// between the control-rate steps, ported from Mozzi's ADSR.h.
//
// Upstream templatizes this on CONTROL_UPDATE_RATE and LERP_RATE (and an
// internal counter type T); this port takes the two rates as constructor
// arguments instead, and always uses a 32 bit step counter -- ample
// headroom on a 32 bit target like the RP2040.
type ADSR struct {
	controlUpdateRate uint32
	lerpsPerControl   uint32

	updateStepCounter uint32
	numUpdateSteps    uint32

	attack, decay, sustain, release, idle adsrPhase
	currentPhase                          *adsrPhase

	transition line.Line[fixmath.Q15n16]
	playing    bool
}

// NewADSR creates an ADSR. controlUpdateRate is normally your engine's
// control rate; lerpRate is how often Next() will be called to interpolate
// between control updates (normally the audio rate, for the smoothest
// result, but it can be the control rate too, if update() and Next() are
// both called from UpdateControl()).
func NewADSR(controlUpdateRate, lerpRate uint32) *ADSR {
	a := &ADSR{
		controlUpdateRate: controlUpdateRate,
		lerpsPerControl:   lerpRate / controlUpdateRate,
	}
	a.attack.kind = phaseAttack
	a.decay.kind = phaseDecay
	a.sustain.kind = phaseSustain
	a.release.kind = phaseRelease
	a.idle.kind = phaseIdle
	a.currentPhase = &a.idle
	return a
}

func (a *ADSR) convertMsecToControlUpdateSteps(msec uint32) uint32 {
	return uint32((uint64(msec) * uint64(a.controlUpdateRate)) >> 10) // approximates /1000
}

func (a *ADSR) setPhase(next *adsrPhase) {
	a.updateStepCounter = 0
	a.numUpdateSteps = next.updateSteps
	a.transition.SetTarget(fixmath.Q8n0ToQ15n16(next.level), next.lerpSteps)
	a.currentPhase = next
}

func (a *ADSR) checkForAndSetNextPhase(next *adsrPhase) {
	a.updateStepCounter++
	if a.updateStepCounter >= a.numUpdateSteps {
		a.setPhase(next)
	}
}

func (a *ADSR) setTime(p *adsrPhase, msec uint32) {
	p.updateSteps = a.convertMsecToControlUpdateSteps(msec)
	p.lerpSteps = int32(p.updateSteps) * int32(a.lerpsPerControl)
}

func (a *ADSR) setUpdateSteps(p *adsrPhase, steps uint32) {
	p.updateSteps = steps
	p.lerpSteps = int32(steps) * int32(a.lerpsPerControl)
}

// Update advances the internal ADSR phase machine. Call this in UpdateControl().
func (a *ADSR) Update() {
	switch a.currentPhase.kind {
	case phaseAttack:
		a.checkForAndSetNextPhase(&a.decay)
	case phaseDecay:
		a.checkForAndSetNextPhase(&a.sustain)
	case phaseSustain:
		a.checkForAndSetNextPhase(&a.release)
	case phaseRelease:
		a.checkForAndSetNextPhase(&a.idle)
	case phaseIdle:
		a.playing = false
	}
}

// Next advances one step along the envelope and returns the level. Call
// this in UpdateAudio() (or UpdateControl(), if lerpRate == controlUpdateRate).
func (a *ADSR) Next() uint8 {
	if !a.playing {
		return 0
	}
	return fixmath.Q15n16ToQ8n0(a.transition.Next())
}

// NoteOn starts the attack phase, continuing from the envelope's current
// level (0 if it was idle). Restarts the envelope no matter what phase it
// was in.
func (a *ADSR) NoteOn() {
	a.setPhase(&a.attack)
	a.playing = true
}

// NoteOnReset is like NoteOn, but forces the envelope to start from 0 even
// if it's still playing -- often more useful for effect envelopes than note envelopes.
func (a *ADSR) NoteOnReset() {
	a.transition.Set(0)
	a.NoteOn()
}

// NoteOff starts the release phase.
func (a *ADSR) NoteOff() { a.setPhase(&a.release) }

// Playing reports whether the envelope is currently playing (i.e. not idle).
func (a *ADSR) Playing() bool { return a.playing }

func (a *ADSR) SetAttackLevel(v uint8)  { a.attack.level = v }
func (a *ADSR) SetDecayLevel(v uint8)   { a.decay.level = v }
func (a *ADSR) SetSustainLevel(v uint8) { a.sustain.level = v }
func (a *ADSR) SetReleaseLevel(v uint8) { a.release.level = v }
func (a *ADSR) SetIdleLevel(v uint8)    { a.idle.level = v }

// SetADLevels sets attack and decay levels for a conventional envelope where
// sustain continues at the decay level until release ramps to (near) 0.
func (a *ADSR) SetADLevels(attack, decay uint8) {
	a.SetAttackLevel(attack)
	a.SetDecayLevel(decay)
	a.SetSustainLevel(decay)
	a.SetReleaseLevel(1)
	a.SetIdleLevel(0)
}

// SetLevels sets all four phase levels directly.
func (a *ADSR) SetLevels(attack, decay, sustain, release uint8) {
	a.SetAttackLevel(attack)
	a.SetDecayLevel(decay)
	a.SetSustainLevel(sustain)
	a.SetReleaseLevel(release)
	a.SetIdleLevel(0)
}

// SetAttackTime sets the attack time in milliseconds (resolved to the
// nearest control-rate step). Beware very small values -- if the internal
// step count rounds to 0, nothing will happen.
func (a *ADSR) SetAttackTime(msec uint32) { a.setTime(&a.attack, msec) }

// SetDecayTime sets the decay time in milliseconds.
func (a *ADSR) SetDecayTime(msec uint32) { a.setTime(&a.decay, msec) }

// SetSustainTime sets the sustain time in milliseconds. The sustain phase
// also ends early if NoteOff() is called.
func (a *ADSR) SetSustainTime(msec uint32) { a.setTime(&a.sustain, msec) }

// SetReleaseTime sets the release time in milliseconds.
func (a *ADSR) SetReleaseTime(msec uint32) { a.setTime(&a.release, msec) }

func (a *ADSR) SetIdleTime(msec uint32) { a.setTime(&a.idle, msec) }

// SetTimes sets attack, decay, sustain and release times in milliseconds all at once.
func (a *ADSR) SetTimes(attackMs, decayMs, sustainMs, releaseMs uint32) {
	a.SetAttackTime(attackMs)
	a.SetDecayTime(decayMs)
	a.SetSustainTime(sustainMs)
	a.SetReleaseTime(releaseMs)
	a.SetIdleTime(65535) // guarantees a step size of 0
}

// SetAttackUpdateSteps sets the attack time as a number of Update() calls, rather than milliseconds.
func (a *ADSR) SetAttackUpdateSteps(steps uint32) { a.setUpdateSteps(&a.attack, steps) }

// SetDecayUpdateSteps sets the decay time as a number of Update() calls.
func (a *ADSR) SetDecayUpdateSteps(steps uint32) { a.setUpdateSteps(&a.decay, steps) }

// SetSustainUpdateSteps sets the sustain time as a number of Update() calls.
func (a *ADSR) SetSustainUpdateSteps(steps uint32) { a.setUpdateSteps(&a.sustain, steps) }

// SetReleaseUpdateSteps sets the release time as a number of Update() calls.
func (a *ADSR) SetReleaseUpdateSteps(steps uint32) { a.setUpdateSteps(&a.release, steps) }

func (a *ADSR) SetIdleUpdateSteps(steps uint32) { a.setUpdateSteps(&a.idle, steps) }

// SetAllUpdateSteps sets attack, decay, sustain and release times as Update() call counts.
func (a *ADSR) SetAllUpdateSteps(attack, decay, sustain, release uint32) {
	a.SetAttackUpdateSteps(attack)
	a.SetDecayUpdateSteps(decay)
	a.SetSustainUpdateSteps(sustain)
	a.SetReleaseUpdateSteps(release)
	a.SetIdleUpdateSteps(65535)
}
