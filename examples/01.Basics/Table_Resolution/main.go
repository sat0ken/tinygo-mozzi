// Command Table_Resolution is a port of Mozzi's
// examples/01.Basics/Table_Resolution: sine tables of six sizes, 256 to
// 8192 cells, each sweeping from 0 to 8192Hz in turn, to hear how table
// size affects aliasing noise (significant at 256 cells, hardly there by
// 1024).
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Table_Resolution
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/eventdelay"
	"mozzi/line"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64

	numOscils            = 6
	millisPerSweep       = 2000
	millisPerControl     = 1000 / controlRate                // 15 (integer division, as upstream)
	controlStepsPerSweep = millisPerSweep / millisPerControl // 133
)

type sketch struct {
	aSin            [numOscils]*oscil.Oscil
	whoseTurn       int // which oscil to listen to
	kWhoseTurnDelay *eventdelay.EventDelay
	kSweep          *line.Line[float32]
}

func (s *sketch) UpdateControl() {
	if s.kWhoseTurnDelay.Ready() {
		s.whoseTurn++
		if s.whoseTurn >= numOscils {
			s.whoseTurn = 0
		}
		s.kWhoseTurnDelay.Start()
		s.kSweep.Set(0)
	}
	s.aSin[s.whoseTurn].SetFreqFloat(s.kSweep.Next())
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aSin[s.whoseTurn].Next()))
}

func main() {
	s := &sketch{
		aSin: [numOscils]*oscil.Oscil{
			oscil.New(tables.Sin256, audioRate),  // can hear significant aliasing noise
			oscil.New(tables.Sin512, audioRate),  // noise still there but less noticeable
			oscil.New(tables.Sin1024, audioRate), // borderline, hardly there if at all
			oscil.New(tables.Sin2048, audioRate), // no audible improvement from here on
			oscil.New(tables.Sin4096, audioRate), // for 45 year old loud sound damaged ears
			oscil.New(tables.Sin8192, audioRate),
		},
		kWhoseTurnDelay: eventdelay.New(millisPerSweep),
		kSweep:          line.New[float32](),
	}
	s.kSweep.SetFromTo(0, 8192, controlStepsPerSweep)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
