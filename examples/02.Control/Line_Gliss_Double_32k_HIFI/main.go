// Command Line_Gliss_Double_32k_HIFI is a port of Mozzi's
// examples/02.Control/Line_Gliss_Double_32k_HIFI: like Line_Gliss, but with
// two sawtooths gliding together, the second detuned by a random amount,
// at a 32768Hz audio rate.
//
// The _HIFI part of upstream's configuration (AVR's two-pin PWM output
// mode) has no equivalent here: the output is chosen by mozzi/board, at
// this port's 8 bit logical resolution (see mozzi/audioout). The 32kHz
// audio rate is kept.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Line_Gliss_Double_32k_HIFI
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/line"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/rand"
	"mozzi/tables"
)

const (
	audioRate   = 32768
	controlRate = 64 // upstream's default

	loNote = 24 // midi note numbers
	hiNote = 46

	audioStepsPerGliss   = audioRate / 4 // ie. 4 glisses per second
	controlStepsPerGliss = controlRate / 4

	glissOffsetMax = 36
)

type sketch struct {
	aSaw1, aSaw2     *oscil.Oscil
	aGliss1, aGliss2 *line.Line[int32]
	counter          int
	glissOffset      uint8
	glissOffsetStep  uint8
}

func variation() int32 {
	return int32(rand.Int16Max(16383)) // upstream's rand(16383): the int overload
}

func (s *sketch) UpdateControl() {
	s.counter--
	if s.counter <= 0 {
		s.glissOffset += s.glissOffsetStep
		if s.glissOffset >= glissOffsetMax {
			s.glissOffset = 0
			if s.glissOffsetStep != 2 {
				s.glissOffsetStep = 2
			} else {
				s.glissOffsetStep = 3
			}
		}
		startFreq := midi.MtoFWhole(loNote + s.glissOffset)
		endFreq := midi.MtoFWhole(hiNote + s.glissOffset)
		glissStart := int32(s.aSaw1.PhaseIncFromFreq(startFreq))
		glissEnd := int32(s.aSaw1.PhaseIncFromFreq(endFreq))
		s.aGliss1.SetFromTo(glissStart, glissEnd, audioStepsPerGliss)
		// Upstream calls variation() twice as arguments of one function call,
		// an order C++ leaves unspecified; Go evaluates left to right.
		offset := int32(s.glissOffset)
		s.aGliss2.SetFromTo((glissStart+variation()*offset)*2, (glissEnd+variation()*offset)*2, audioStepsPerGliss)
		s.counter = controlStepsPerGliss
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	s.aSaw1.SetPhaseInc(uint32(s.aGliss1.Next()))
	s.aSaw2.SetPhaseInc(uint32(s.aGliss2.Next()))
	return audioout.FromNBit(9, int32(s.aSaw1.Next())+int32(s.aSaw2.Next()))
}

func main() {
	s := &sketch{
		aSaw1:           oscil.New(tables.Saw8192, audioRate),
		aSaw2:           oscil.New(tables.Saw8192, audioRate),
		aGliss1:         line.New[int32](),
		aGliss2:         line.New[int32](),
		glissOffsetStep: 2,
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
