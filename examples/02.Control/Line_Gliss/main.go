// Command Line_Gliss is a port of Mozzi's examples/02.Control/Line_Gliss:
// a sawtooth gliding smoothly between frequencies, four glissandos a
// second. Instead of recalculating the frequency at every step, a Line
// interpolates the oscillator's phase increment at audio rate, with one
// addition per sample.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Line_Gliss
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/line"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64

	loNote = 24 // midi note numbers
	hiNote = 36

	audioStepsPerGliss   = audioRate / 4 // ie. 4 glisses per second
	controlStepsPerGliss = controlRate / 4

	glissOffsetStep = 2
	glissOffsetMax  = 36
)

type sketch struct {
	aSaw        *oscil.Oscil
	aGliss      *line.Line[int32]
	counter     int
	glissOffset uint8
}

func (s *sketch) UpdateControl() {
	s.counter--
	if s.counter <= 0 {
		s.glissOffset += glissOffsetStep
		if s.glissOffset >= glissOffsetMax {
			s.glissOffset = 0
		}
		startFreq := midi.MtoFWhole(loNote + s.glissOffset)
		endFreq := midi.MtoFWhole(hiNote + s.glissOffset)
		glissStart := int32(s.aSaw.PhaseIncFromFreq(startFreq))
		glissEnd := int32(s.aSaw.PhaseIncFromFreq(endFreq))
		s.aGliss.SetFromTo(glissStart, glissEnd, audioStepsPerGliss)
		s.counter = controlStepsPerGliss
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	s.aSaw.SetPhaseInc(uint32(s.aGliss.Next()))
	return audioout.From8Bit(int16(s.aSaw.Next()))
}

func main() {
	s := &sketch{
		aSaw:   oscil.New(tables.Saw8192, audioRate),
		aGliss: line.New[int32](),
	}

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
