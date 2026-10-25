// Command Control_Echo_Theremin is a port of Mozzi's
// examples/02.Control/Control_Echo_Theremin: a theremin-like instrument
// played by moving your hand over a light sensor, with long echoes. The
// sensor reading (smoothed with a RollingAverage) sets an oscillator's
// frequency, and a control-rate delay line (upstream's ControlDelay) feeds
// three more oscillators the same frequency 0.5, 1 and 2 seconds later.
//
// The input is mozzi/board's A0. On the TinyGo Conf 2026 badge that's the
// joystick's X axis, so you play it by moving the stick (centred is about
// 512Hz). Elsewhere, wire a light-dependent resistor (LDR) from 3.3V to A0
// and a fixed resistor (e.g. 10k) from A0 to GND, so the voltage on A0 rises
// with the light level. On the M5Stack CoreS3, A0 is GPIO8 on Port B --
// take the 3.3V from the M-Bus header, not Port B's 5V, which the
// ESP32-S3's pins can't take. Unconnected, the input floats and the pitch
// wanders.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Control_Echo_Theremin
package main

import (
	"mozzi/analog"
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/delay"
	"mozzi/engine"
	"mozzi/oscil"
	"mozzi/rollingaverage"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64

	inputPin = board.A0 // analog control input

	echoCells1 = 32
	echoCells2 = 60
	echoCells3 = 127
)

type sketch struct {
	aSin0, aSin1, aSin2, aSin3 *oscil.Oscil
	// kDelay is upstream's ControlDelay<128, int>, which is just an
	// AudioDelay used at control rate: 128 cells = 2 seconds at 64Hz.
	kDelay   *delay.AudioDelay[int32]
	kAverage *rollingaverage.RollingAverage[int32] // how many to average has to be a power of 2
	averaged int32
}

func (s *sketch) UpdateControl() {
	bumpyInput := int32(analog.Read(inputPin, 10)) // reading at 10-bit resolution, i.e. 0-1023
	s.averaged = s.kAverage.Next(bumpyInput)
	s.aSin0.SetFreq(int(s.averaged))
	s.aSin1.SetFreq(int(s.kDelay.Next(s.averaged)))
	s.aSin2.SetFreq(int(s.kDelay.Read(echoCells2)))
	s.aSin3.SetFreq(int(s.kDelay.Read(echoCells3)))
}

func (s *sketch) UpdateAudio() audioout.Mono {
	sum := int32(s.aSin0.Next()) + int32(s.aSin1.Next()) + int32(s.aSin2.Next()>>1) + int32(s.aSin3.Next()>>2)
	return audioout.FromAlmostNBit(12, 3*sum)
}

func main() {
	s := &sketch{
		aSin0:    oscil.New(tables.Sin2048, audioRate),
		aSin1:    oscil.New(tables.Sin2048, audioRate),
		aSin2:    oscil.New(tables.Sin2048, audioRate),
		aSin3:    oscil.New(tables.Sin2048, audioRate),
		kDelay:   delay.New[int32](128),
		kAverage: rollingaverage.New[int32](32),
	}
	s.kDelay.Set(echoCells1)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
