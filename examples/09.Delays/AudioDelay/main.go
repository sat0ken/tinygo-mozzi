// Command AudioDelay is a port of Mozzi's examples/09.Delays/AudioDelay:
// a signal put through a smoothly modulated delay line (a "doubler"/flanger
// effect).
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/09.Delays/AudioDelay
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/delay"
	"mozzi/engine"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 256 // Hz, matches upstream's AudioDelay example
)

type sketch struct {
	aTriangle    *oscil.Oscil
	kFreq        *oscil.Oscil
	aDel         *delay.AudioDelay[int8]
	delaySamples uint32
}

func newSketch() *sketch {
	s := &sketch{
		aTriangle: oscil.New(tables.TriangleAnalogue512, audioRate),
		kFreq:     oscil.New(tables.Cos2048, controlRate),
		aDel:      delay.New[int8](256),
	}
	s.aTriangle.SetFreq(midi.MtoFWhole(60))
	s.kFreq.SetFreqFloat(0.63)
	return s
}

func (s *sketch) UpdateControl() {
	s.delaySamples = uint32(128 + int32(s.kFreq.Next()))
}

func (s *sketch) UpdateAudio() audioout.Mono {
	asig := s.aDel.NextWithDelay(s.aTriangle.Next(), s.delaySamples)
	return audioout.From8Bit(int16(asig))
}

func main() {
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
