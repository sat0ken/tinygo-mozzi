// Command Sinewave is a port of Mozzi's examples/01.Basics/Sinewave: play a
// single sine wave at a fixed frequency. The simplest possible sketch,
// meant as a first "does my wiring/build work" check.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384 // Hz -- matches classic Mozzi STANDARD mode
	controlRate = 64    // Hz
)

// sketch mirrors a classic Mozzi .ino's global Oscil + updateControl/updateAudio pair.
type sketch struct {
	aSin *oscil.Oscil
}

func (s *sketch) UpdateControl() {
	// put changing controls in here
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aSin.Next())) // an int8 signal centred around 0
}

func main() {
	s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
	s.aSin.SetFreq(440) // set the frequency

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook() // required here
	}
}
