// Package engine is a port of Mozzi's MozziGuts.hpp/AudioOutput.h control
// flow: the scheduler that ties a sketch's UpdateControl/UpdateAudio
// functions to a hardware audio backend.
//
// Basic architecture (mirrors upstream's "Basic architecture of audio
// generation, buffering, and output" doc in AudioOutput.h):
//
//  1. Your program's main loop calls AudioHook() as fast as it can.
//  2. If the output ring buffer has room, AudioHook() calls UpdateControl()
//     (throttled to your configured control rate) and UpdateAudio() (every
//     time), pushing the resulting sample into the buffer.
//  3. Independently, the hardware Backend drives real time: at your
//     configured audio rate, it pulls one sample from the buffer and writes
//     it to the DAC/PWM/etc, usually from a timer interrupt.
//
// This decouples "how long generating a sample takes" from "exactly when a
// sample must reach the hardware" the same way upstream's CircularBuffer +
// per-platform timer ISR does.
//
// AudioHook() also yields to other goroutines on every call (see its doc).
// This matters for backends like mozzi/backend/rp2040i2s that pace output
// from a goroutine rather than a hardware interrupt: TinyGo's cooperative
// scheduler only switches goroutines at an explicit yield point, so without
// this, a tight `for { engine.AudioHook() }` main loop -- which never
// blocks on anything by design -- would monopolize the CPU and the
// output-writing goroutine would never run at all, producing total
// silence. Timer-interrupt-driven backends (rp2040pwm, esp32s3pwm) don't
// need this (a real interrupt preempts regardless of goroutine scheduling),
// but the yield is cheap enough there to not be worth special-casing away.
package engine

import (
	"mozzi/audioout"
	"runtime"
)

// Sketch is implemented by a user program. It plays the role of the
// updateControl()/updateAudio() pair of free functions every Mozzi sketch
// defines upstream.
type Sketch interface {
	// UpdateControl is called at the configured control rate. Do your
	// slower, non-audio-rate work here (reading sensors, envelopes,
	// updating oscillator frequencies, etc).
	UpdateControl()
	// UpdateAudio is called once per audio sample. Keep this fast and
	// allocation-free.
	UpdateAudio() audioout.Mono
}

// Backend abstracts the platform-specific audio hardware.
// See mozzi/backend/rp2040pwm for the Raspberry Pi Pico PWM implementation.
type Backend interface {
	// Start begins driving audio hardware at sampleRateHz, calling pull
	// once per sample -- typically from a timer interrupt -- to obtain the
	// next sample to output. pull is safe to call from interrupt context;
	// implementations must not block or allocate in it.
	Start(sampleRateHz uint32, pull func() audioout.Storage)
}

var (
	sketch  Sketch
	backend Backend

	buf ringBuffer[audioout.Storage]

	updateControlTimeout uint32
	updateControlCounter uint32

	currentAudioRate uint32

	lastSample audioout.Storage
)

// Stopper is implemented by backends that can stop their audio output
// (see Stop). All of this port's backends do.
type Stopper interface {
	// Stop stops calling pull and outputting samples, until the backend's
	// Start is called again.
	Stop()
}

// Stop stops audio output, mirroring upstream's stopMozzi(): the backend
// stops pulling samples (if it implements Stopper; otherwise this does
// nothing), so there's nothing for AudioHook to do until Start is called
// again to resume -- with the same arguments as before, typically.
func Stop() {
	if st, ok := backend.(Stopper); ok {
		st.Stop()
	}
}

// Start configures and starts the engine, mirroring upstream's startMozzi().
// audioRateHz is how often UpdateAudio runs (and samples are output);
// controlRateHz is how often UpdateControl runs. Call this once from your
// program's main(), then call AudioHook() in a tight loop. It can be called
// again after Stop, to resume.
func Start(s Sketch, audioRateHz, controlRateHz uint32, b Backend) {
	sketch = s
	backend = b
	currentAudioRate = audioRateHz
	if controlRateHz == 0 {
		controlRateHz = 1
	}
	updateControlTimeout = currentAudioRate/controlRateHz - 1
	b.Start(currentAudioRate, pullSample)
}

// AudioHook fills the output buffer while there's room in it, running a
// control update (at the configured rate) and generating one audio sample
// each time through. Call this as often as possible from main(); it never
// blocks, and does nothing (but still yield -- see below) when the buffer
// is already full.
//
// Every call ends with runtime.Gosched(), unconditionally, so that other
// goroutines -- notably a goroutine-driven Backend's output loop -- get a
// chance to run. See the package doc for why this is necessary.
func AudioHook() {
	if !buf.isFull() {
		advanceControlLoop()
		buf.write(sketch.UpdateAudio().L())
	}
	runtime.Gosched()
}

func advanceControlLoop() {
	if updateControlCounter == 0 {
		updateControlCounter = updateControlTimeout
		sketch.UpdateControl()
	} else {
		updateControlCounter--
	}
}

// pullSample is called from the backend's interrupt to obtain the next
// sample. If the buffer has momentarily run dry, it repeats the last sample
// rather than glitching to silence or garbage.
func pullSample() audioout.Storage {
	if buf.isEmpty() {
		return lastSample
	}
	lastSample = buf.read()
	return lastSample
}

// AudioTicks returns the number of audio samples output so far, mirroring
// upstream's audioTicks(). Used by mozzi/eventdelay and friends to schedule
// events without depending on wall-clock time.
func AudioTicks() uint64 { return buf.count() }

// AudioRate returns the audio rate the engine was started with.
func AudioRate() uint32 { return currentAudioRate }

// Micros returns an approximation of elapsed microseconds since Start,
// derived from AudioTicks, mirroring upstream's mozziMicros().
func Micros() uint64 {
	if currentAudioRate == 0 {
		return 0
	}
	return AudioTicks() * 1000000 / uint64(currentAudioRate)
}
