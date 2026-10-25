// Command FMsynth is a port of Mozzi's examples/06.Synthesis/FMsynth:
// simple FM synthesis via phase modulation, with a slowly wandering pitch
// and a modulation index that breathes over time. Also a demonstration of
// this port's fixmath package for fractional-frequency fixed-point math.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/06.Synthesis/FMsynth
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/envelope"
	"mozzi/eventdelay"
	"mozzi/fixmath"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/rand"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 256 // Hz, matches upstream's FMsynth example
)

// modToCarrierRatio is the FM ratio between oscillator frequencies; it
// stays constant across the note range. float_to_Q8n8(3.0).
const modToCarrierRatio fixmath.Q8n8 = 3 << 8

type sketch struct {
	aCarrier, aModulator, kModIndex *oscil.Oscil
	noteDelay                       *eventdelay.EventDelay
	smoothNote                      *envelope.SmoothInt32

	modIndex             fixmath.Q8n8
	deviation            fixmath.Q16n16 // really Q24n8-scaled (same uint32 storage either way); see setFreqs
	carrierFreq, modFreq fixmath.Q16n16

	attack, decay, sustain, release uint32

	note0, note1, targetNote, noteChangeStep, noteLowerLimit, noteUpperLimit fixmath.Q7n8
}

func newSketch() *sketch {
	s := &sketch{
		aCarrier:   oscil.New(tables.Cos2048, audioRate),
		aModulator: oscil.New(tables.Cos2048, audioRate),
		kModIndex:  oscil.New(tables.Cos2048, controlRate),
		noteDelay:  eventdelay.New(0),
		smoothNote: envelope.NewSmoothInt32(0.95),
	}
	s.kModIndex.SetFreqFloat(0.768) // sync with the note-change delay, below

	s.noteChangeStep = fixmath.Q7n0ToQ7n8(3)
	s.noteUpperLimit = fixmath.Q7n0ToQ7n8(50)
	s.noteLowerLimit = fixmath.Q7n0ToQ7n8(32)
	s.note0 = s.noteLowerLimit
	s.note1 = s.noteLowerLimit + fixmath.Q7n0ToQ7n8(5)
	s.targetNote = s.note0

	s.noteDelay.StartFor(768) // ms countdown, tailored to controlRate's resolution
	return s
}

func (s *sketch) setFreqs(midiNote fixmath.Q8n8) {
	s.carrierFreq = midi.Q16n16MtoF(fixmath.Q8n8ToQ16n16(midiNote))
	s.modFreq = (s.carrierFreq >> 8) * uint32(modToCarrierRatio) // (Q16n16>>8) * Q8n8 = Q16n16; beware overflow, as upstream notes
	s.deviation = (s.modFreq >> 16) * uint32(s.modIndex)
	s.aCarrier.SetFreqQ16n16(s.carrierFreq)
	s.aModulator.SetFreqQ16n16(s.modFreq)
}

func (s *sketch) UpdateControl() {
	if s.noteDelay.Ready() {
		if s.targetNote == s.note0 {
			s.note1 += s.noteChangeStep
			s.targetNote = s.note1
		} else {
			s.note0 += s.noteChangeStep
			s.targetNote = s.note0
		}

		if s.note0 > s.noteUpperLimit {
			s.noteChangeStep = fixmath.Q7n0ToQ7n8(-3)
		}
		if s.note0 < s.noteLowerLimit {
			s.noteChangeStep = fixmath.Q7n0ToQ7n8(3)
		}

		s.noteDelay.StartFor(uint32(s.attack + s.decay + s.sustain + s.release))
	}

	// vary the modulation index
	s.modIndex = fixmath.Q8n8(350 + int32(s.kModIndex.Next()))

	// smooth note changes, to avoid zipper noise
	s.targetNoteSmoothed()
}

func (s *sketch) targetNoteSmoothed() {
	smoothed := s.smoothNote.Next(int32(s.targetNote))
	s.setFreqs(fixmath.Q8n8(smoothed))
}

func (s *sketch) UpdateAudio() audioout.Mono {
	modulation := fixmath.Q15n16((int64(s.deviation) * int64(s.aModulator.Next())) >> 8)
	return audioout.From8Bit(int16(s.aCarrier.PhMod(modulation)))
}

func main() {
	rand.Seed(0xC0FFEE) // fresh-ish sequence each build; swap for a hardware RNG seed if you have one
	s := newSketch()
	s.attack, s.decay, s.sustain, s.release = 100, 100, 100, 100

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
