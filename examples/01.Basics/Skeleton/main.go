// Command Skeleton is a port of Mozzi's examples/01.Basics/Skeleton: the
// minimal shape every sketch built on this port has, without any actual
// sound generation. Copy this directory as a starting point for a new sketch.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Skeleton
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
)

const (
	audioRate   = 16384
	controlRate = 64
)

type sketch struct{}

func (s *sketch) UpdateControl() {
	// your control code
}

func (s *sketch) UpdateAudio() audioout.Mono {
	// For mono output, this just needs to return a signed sample. Pick
	// whichever of these matches the "natural" range of the values you
	// generate, for the clearest and most portable code:
	return audioout.From8Bit(0) // if your signal is in 8 bit range
	/* OR */
	return audioout.FromAlmostNBit(9, 0).Clip() // if your signal is between -244 and 243 (_almost_ 9 bits is a typical result of adding several 8 bit signals); Clip() instead of overflowing, should a few stray values exceed the range
	/* OR */
	return audioout.FromNBit(21, 0) // if your signal happens to be in 21 bit range
}

func main() {
	s := &sketch{}

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook() // fills the audio buffer
	}
}
