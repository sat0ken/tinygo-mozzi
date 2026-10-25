// Package metronome is a port of Mozzi's Metronome.h: an EventDelay that
// repeats, firing every n milliseconds (or at a given BPM) without
// drifting, timed against the engine's audio-tick counter.
package metronome

import "mozzi/engine"

// Metronome fires (Ready returns true) once every period, starting one
// period after Start. Like upstream it subtracts any lateness when it
// re-arms, so the beat doesn't slip if Ready is polled a little late.
type Metronome struct {
	millis   uint32
	deadline uint64
	stopped  bool
}

// New creates a Metronome with a period of delayMilliseconds. Like
// upstream's, it's running from the start (Ready fires at once, then every
// period); call Start to (re)start it from now.
func New(delayMilliseconds uint32) *Metronome {
	m := &Metronome{}
	m.Set(delayMilliseconds)
	return m
}

// Set sets the period, in milliseconds.
func (m *Metronome) Set(delayMilliseconds uint32) { m.millis = delayMilliseconds }

// ticks converts the period to audio ticks. Done on use rather than in Set,
// so a Metronome can be created before engine.Start (upstream's audio rate
// is a compile-time constant; here it's only known once the engine runs).
func (m *Metronome) ticks() uint64 {
	return uint64(m.millis) * uint64(engine.AudioRate()) / 1000
}

// SetBPM sets the period from a tempo in beats per minute.
func (m *Metronome) SetBPM(bpm float32) { m.Set(uint32(60000 / bpm)) }

// Start (re)starts the Metronome: it fires one period from now.
func (m *Metronome) Start() {
	m.deadline = engine.AudioTicks() + m.ticks()
	m.stopped = false
}

// StartFor sets the period and starts the Metronome.
func (m *Metronome) StartFor(delayMilliseconds uint32) {
	m.Set(delayMilliseconds)
	m.Start()
}

// Stop stops the Metronome; Ready returns false until Start is called.
func (m *Metronome) Stop() { m.stopped = true }

// Ready reports whether a beat is due, and if so, schedules the next one.
// Call it from UpdateControl.
func (m *Metronome) Ready() bool {
	now := engine.AudioTicks()
	if now < m.deadline || m.stopped {
		return false
	}
	m.deadline += m.ticks() // = now - (now-deadline) + ticks: the overrun is subtracted
	return true
}
