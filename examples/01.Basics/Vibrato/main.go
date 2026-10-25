// Command Vibrato is a port of Mozzi's examples/01.Basics/Vibrato:
// simple FM/vibrato using phase modulation (Oscil.PhMod).
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Vibrato
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64
)

type sketch struct {
	aCos, aVibrato *oscil.Oscil
}

func (s *sketch) UpdateControl() {}

func (s *sketch) UpdateAudio() audioout.Mono {
	// Upstream: intensity (UFix<0,8>, 0.5 = raw 128) * toSFraction(sample)
	// (SFix<0,7>, raw = the int8 sample) is an SFix<0,15>, which phMod
	// widens to the Q15n16 proportion of the table length it works in, i.e.
	// raw << 1. Altogether: sample * 256 -- the phase is modulated by up to
	// half a table length either way.
	vibrato := int32(s.aVibrato.Next()) * 128 << 1
	return audioout.From8Bit(int16(s.aCos.PhMod(vibrato)))
}

func main() {
	s := &sketch{
		aCos:     oscil.New(tables.Cos2048, audioRate),
		aVibrato: oscil.New(tables.Cos2048, audioRate),
	}
	s.aCos.SetFreqFloat(midi.MtoF(84)) // upstream: setFreq(mtof(84.f)), i.e. the float versions
	s.aVibrato.SetFreqFloat(15)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
