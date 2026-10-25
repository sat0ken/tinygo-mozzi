// Command Control_Oscil_Wash is a port of Mozzi's
// examples/02.Control/Control_Oscil_Wash: a fluctuating ambient wash of
// eight cosine tones, each with its volume slowly swept up and down by its
// own control-rate oscillator.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Control_Oscil_Wash
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
	controlRate = 128
)

// The audio oscillators' MIDI notes, and their volume oscillators' frequencies.
var (
	notes    = [8]uint8{60, 74, 64, 77, 67, 81, 60, 84}
	volFreqs = [8]float32{
		4.43, // more of a pulse
		0.0245, 0.019, 0.07, 0.047, 0.031, 0.0717, 0.041,
	}
)

type sketch struct {
	aCos [8]*oscil.Oscil // audio
	kVol [8]*oscil.Oscil // control: volume of each aCos
	v    [8]int8         // current volumes
}

func (s *sketch) UpdateControl() {
	s.v[0] = s.kVol[0].Next() >> 1 // going at a higher freq, this creates zipper noise, so reduce the gain
	for i := 1; i < len(s.v); i++ {
		s.v[i] = s.kVol[i].Next()
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	var asig int32
	for i, osc := range s.aCos {
		asig += int32(osc.Next()) * int32(s.v[i])
	}
	return audioout.FromAlmostNBit(18, asig)
}

func main() {
	s := &sketch{}
	for i := range s.aCos {
		s.aCos[i] = oscil.New(tables.Cos8192, audioRate)
		s.aCos[i].SetFreq(midi.MtoFWhole(notes[i])) // upstream: mtof(int), the whole-note table lookup
		s.kVol[i] = oscil.New(tables.Cos8192, controlRate)
		s.kVol[i].SetFreqFloat(volFreqs[i])
		s.v[i] = 127
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
