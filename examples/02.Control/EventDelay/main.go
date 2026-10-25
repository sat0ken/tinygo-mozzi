// Command EventDelay is a port of Mozzi's examples/02.Control/EventDelay: a
// tone toggled on and off every second by an EventDelay, Mozzi's
// non-blocking replacement for delay().
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/EventDelay
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/eventdelay"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64
)

type sketch struct {
	aSin             *oscil.Oscil
	kGainChangeDelay *eventdelay.EventDelay
	gain             int8
}

func (s *sketch) UpdateControl() {
	if s.kGainChangeDelay.Ready() {
		s.gain = 1 - s.gain // flip 0/1
		s.kGainChangeDelay.Start()
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aSin.Next()) * int16(s.gain))
}

func main() {
	s := &sketch{
		aSin:             oscil.New(tables.Sin8192, audioRate),
		kGainChangeDelay: eventdelay.New(1000), // 1 second countdown, within resolution of controlRate
		gain:             1,
	}
	s.aSin.SetFreq(330) // set the frequency

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
