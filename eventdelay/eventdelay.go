// Package eventdelay is a port of Mozzi's EventDelay.h: a non-blocking
// replacement for delay(), timed against the engine's audio-tick counter
// rather than wall-clock time.
package eventdelay

import "mozzi/engine"

// EventDelay can be Set to a number of milliseconds, then, after Start, its
// Ready method returns true once the time is up. StartFor combines Set and
// Start.
type EventDelay struct {
	millis   uint32
	deadline uint64
}

// New creates an EventDelay, optionally with an initial delay in
// milliseconds (pass 0 and call Set later if you don't know it yet).
func New(delayMilliseconds uint32) *EventDelay {
	e := &EventDelay{}
	e.Set(delayMilliseconds)
	return e
}

// Set sets the delay time. This persists until Set is called again.
func (e *EventDelay) Set(delayMilliseconds uint32) { e.millis = delayMilliseconds }

// Start (re)starts the delay from now. The delay is converted to audio
// ticks here rather than in Set, so an EventDelay can be created (and Set)
// before engine.Start: upstream's audio rate is a compile-time constant,
// this port's is only known once the engine is running.
func (e *EventDelay) Start() {
	e.deadline = engine.AudioTicks() + uint64(e.millis)*uint64(engine.AudioRate())/1000
}

// StartFor sets the delay time and starts it in one call.
func (e *EventDelay) StartFor(delayMilliseconds uint32) {
	e.Set(delayMilliseconds)
	e.Start()
}

// Ready reports whether the delay time is up. Call this from
// UpdateControl() or UpdateAudio().
func (e *EventDelay) Ready() bool { return engine.AudioTicks() >= e.deadline }
