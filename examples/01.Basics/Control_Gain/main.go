// Command Control_Gain is a port of Mozzi's examples/01.Basics/Control_Gain:
// a sine wave whose gain (volume) is changed at control rate -- it keeps
// stepping down, wrapping round from silent back to full volume.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Control_Gain
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64 // upstream's default
)

type sketch struct {
	aSin *oscil.Oscil
	gain uint8
}

func (s *sketch) UpdateControl() {
	s.gain -= 3 // wraps round, like upstream's byte
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From16Bit(int32(s.aSin.Next()) * int32(s.gain)) // 8 bits waveform * 8 bits gain makes 16 bits
}

func main() {
	s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate), gain: 255}
	s.aSin.SetFreq(3320) // set the frequency

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook() // required here
	}
}
