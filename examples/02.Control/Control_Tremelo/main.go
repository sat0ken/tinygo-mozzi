// Command Control_Tremelo is a port of Mozzi's
// examples/02.Control/Control_Tremelo: tremolo (amplitude modulation) from
// a control-rate sine oscillator, with the gain interpolated at audio rate
// by a Line so the modulation is smooth rather than stepped.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Control_Tremelo
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
)

type sketch struct {
	aSig     *oscil.Oscil // audio
	kTremelo *oscil.Oscil // control: the tremolo
	aGain    *line.Line[uint32]
}

func (s *sketch) UpdateControl() {
	gain := uint32(128+int32(s.kTremelo.Next())) << 8
	s.aGain.SetTarget(gain, audioRate/controlRate)
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.FromNBit(24, int32(s.aSig.Next())*int32(s.aGain.Next())) // shifted back to audio range after multiply
}

func main() {
	s := &sketch{
		aSig:     oscil.New(tables.TriangleValve2048, audioRate),
		kTremelo: oscil.New(tables.Sin2048, controlRate),
		aGain:    line.New[uint32](),
	}
	s.aSig.SetFreqFloat(midi.MtoF(65))
	s.kTremelo.SetFreqFloat(5.5)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
