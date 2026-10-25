// Command ResonantFilter is a port of Mozzi's
// examples/10.Audio_Filters/ResonantFilter: filtering a recorded-audio
// wavetable with a resonant low-pass filter whose cutoff wanders slowly.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/10.Audio_Filters/ResonantFilter
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/filter"
	"mozzi/oscil"
	"mozzi/rand"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64

	resonance uint8 = 200 // 0-255, 255 is most resonant
)

type sketch struct {
	aCrunchySound *oscil.Oscil
	kFilterMod    *oscil.Oscil
	rf            *filter.Filter8
}

func newSketch() *sketch {
	s := &sketch{
		aCrunchySound: oscil.New(tables.Chum9, audioRate),
		kFilterMod:    oscil.New(tables.Cos2048, controlRate),
		rf:            filter.NewFilter8(),
	}
	s.aCrunchySound.SetFreqFloat(2)
	s.kFilterMod.SetFreqFloat(1.3)
	return s
}

func (s *sketch) UpdateControl() {
	if rand.Uint16Max(controlRate/2) == 0 { // about once every half second
		s.kFilterMod.SetFreqFloat(float32(rand.Int16Max(255)) / 64) // Int16Max: upstream's rand(255) is the rand(int) overload
	}
	// map the modulation into the filter range (0-255)
	cutoff := uint8(100 + int32(s.kFilterMod.Next())/2)
	s.rf.SetCutoffFreqAndResonance(cutoff, resonance)
}

func (s *sketch) UpdateAudio() audioout.Mono {
	s.rf.Next(int32(s.aCrunchySound.Next()))
	return audioout.From8Bit(int16(s.rf.Low()))
}

func main() {
	s := newSketch()

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
