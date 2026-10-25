// Command Skeleton_Multi is a port of Mozzi's
// examples/01.Basics/Skeleton_Multi: the shape of a sketch split over more
// than one source file. Upstream needs care for that in C++ (Mozzi.h
// included in exactly one file, MozziHeadersOnly.h in the others); in Go,
// a sketch is just package main, and its files -- here main.go and
// unit2.go, which holds UpdateAudio -- can be split up however you like.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Skeleton_Multi
package main

import (
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

// UpdateAudio is in unit2.go.

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
