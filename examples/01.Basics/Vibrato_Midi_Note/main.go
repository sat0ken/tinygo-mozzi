// Command Vibrato_Midi_Note is a port of Mozzi's
// examples/01.Basics/Vibrato_Midi_Note: vibrato done by modulating the
// MIDI note number (by up to 2.5 semitones either way) at control rate and
// converting it to a frequency, rather than by phase modulation as in
// Vibrato.
//
// Upstream writes this with the FixMath library's UFix/SFix types; this
// port does the same fixed-point arithmetic on plain integers, with each
// step's format noted, and ends up passing midi.Q16n16MtoF exactly the
// value upstream's mtof() passes to Q16n16_mtof.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Vibrato_Midi_Note
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/fixmath"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate = 16384
	// controlRate is higher than in Vibrato, because the modulation is
	// done at control rate.
	controlRate = 256

	midiNote = 65 // UFix<7,0>
	// modAmplitude is the vibrato depth in semitones, 2.5: upstream's
	// UFix<2,1>, i.e. raw value 5 with one fractional bit.
	modAmplitudeRaw = 5
)

type sketch struct {
	aCos, kVibrato *oscil.Oscil
}

func (s *sketch) UpdateControl() {
	// toSFraction(kVibrato.next()) is an SFix<0,7> (raw = the int8 sample);
	// times mod_amplitude it's an SFix<2,8> (raw = sample * 5).
	modulation := int32(s.kVibrato.Next()) * modAmplitudeRaw
	// midi_note + modulation, in 8 fractional bits, then widened to
	// Q16n16 -- what mtof(SFix) hands to Q16n16_mtof.
	note := fixmath.Q16n16((midiNote<<8 + modulation) << 8)
	s.aCos.SetFreqQ16n16(midi.Q16n16MtoF(note))
}

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(int16(s.aCos.Next()))
}

func main() {
	s := &sketch{
		aCos:     oscil.New(tables.Cos2048, audioRate),
		kVibrato: oscil.New(tables.Cos2048, controlRate),
	}
	s.aCos.SetFreqQ16n16(midi.Q16n16MtoF(midiNote << 16)) // mtof(UFix<7,0>): the table value for the whole note
	s.kVibrato.SetFreqQ16n16(10 << 16)                    // frequency of the modulation: UFix<16,16>(10)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
