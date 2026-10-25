// Command Sinewave_HIFI is a port of Mozzi's examples/01.Basics/Sinewave_HIFI:
// a sine wave at a fixed frequency, like Sinewave.
//
// Upstream's _HIFI examples differ only in their configuration: they select
// AVR's two-pin PWM output mode (MOZZI_OUTPUT_2PIN_PWM), which combines two
// pins through resistors for about 14 bits of output instead of 8. Here the
// output is chosen by mozzi/board instead, and this port's logical output
// resolution is 8 bits on every board (see mozzi/audioout), so this plays
// exactly like Sinewave.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave_HIFI
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
}

func (s *sketch) UpdateControl() {
	// put changing controls in here
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aSin.Next()))
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
