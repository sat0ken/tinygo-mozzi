// Command ADSR_Audio_Rate_Envelope is a port of Mozzi's
// examples/07.Envelopes/ADSR_Audio_Rate_Envelope: a tone whose pitch and
// ADSR envelope shape are randomized every couple of seconds, with the
// envelope updated at audio rate for fast, click-free transitions.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/07.Envelopes/ADSR_Audio_Rate_Envelope
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/envelope"
	"mozzi/eventdelay"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/rand"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64
)

type sketch struct {
	aOscil    *oscil.Oscil
	env       *envelope.ADSR
	noteDelay *eventdelay.EventDelay

	attack, decay, sustain, release uint32
}

func newSketch() *sketch {
	s := &sketch{
		aOscil:    oscil.New(tables.Sin8192, audioRate),
		env:       envelope.NewADSR(audioRate, audioRate), // both rates = audioRate, as in upstream's audio-rate ADSR example
		noteDelay: eventdelay.New(0),
	}
	s.noteDelay.StartFor(2000) // 2 second countdown
	return s
}

func (s *sketch) UpdateControl() {
	if !s.noteDelay.Ready() {
		return
	}

	// Int16Max, not Uint8Max: upstream passes int literals, which picks
	// Mozzi's rand(int) overload (a different formula from rand(uint8_t)).
	attackLevel := uint8(rand.Int16Max(128)) + 127
	decayLevel := uint8(rand.Int16Max(255))
	s.env.SetADLevels(attackLevel, decayLevel)

	newValue := uint32(rand.Uint16Max(300)) + 100
	switch rand.Int16Max(4) {
	case 0:
		s.attack = newValue
	case 1:
		s.decay = newValue
	case 2:
		s.sustain = newValue
	case 3:
		s.release = newValue
	}
	s.env.SetTimes(s.attack, s.decay, s.sustain, s.release)
	s.env.NoteOn()

	midiNote := uint8(rand.Int16Max(107)) + 20
	s.aOscil.SetFreq(midi.MtoFWhole(midiNote))

	s.noteDelay.StartFor(s.attack + s.decay + s.sustain + s.release)
}

func (s *sketch) UpdateAudio() audioout.Mono {
	s.env.Update()
	return audioout.From16Bit(int32(s.env.Next()) * int32(s.aOscil.Next()))
}

func main() {
	rand.Seed(0x5EED1)
	s := newSketch()

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
