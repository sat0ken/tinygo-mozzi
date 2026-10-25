// Command Stop_Start is a port of Mozzi's examples/02.Control/Stop_Start:
// press a button to stop Mozzi (engine.Stop, upstream's stopMozzi) for
// about a second, counting on the serial console meanwhile, then start it
// again (engine.Start). Upstream's ESP32 port leaves stopMozzi
// unimplemented; here every backend supports it.
//
// The button is mozzi/board's Button (upstream reads digital pin 4). On the
// TinyGo Conf 2026 badge that's SW1, already there. Elsewhere, wire a push
// button from the Button pin to 3.3V -- on the M5Stack CoreS3 that's GPIO9
// on Port B; take the 3.3V from the M-Bus header, not Port B's 5V, which
// the ESP32-S3's pins can't take. Watch the count with:
//
//	tinygo monitor -port /dev/ttyACM0
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Stop_Start
package main

import (
	"time"

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
	aSin           *oscil.Oscil
	pauseTriggered bool
	count          uint
}

func (s *sketch) UpdateControl() {
	if board.ButtonPressed() {
		s.pauseTriggered = true
		s.count = 0
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aSin.Next()))
}

func main() {
	board.ConfigureButton()
	s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
	s.aSin.SetFreq(220) // set the frequency

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	paused := false
	for {
		if s.pauseTriggered {
			paused = true
			s.pauseTriggered = false
			engine.Stop()
		}
		if paused {
			println(s.count)
			s.count++
			time.Sleep(100 * time.Millisecond)
			if s.count > 10 {
				paused = false
				engine.Start(s, audioRate, controlRate, out)
			}
		}
		if !paused {
			engine.AudioHook()
		}
	}
}
