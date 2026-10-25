// Command fanfare_synth is a port of a hand-written Arduino/Mozzi sketch
// (mozzi_max98357a_synth_melody_esp32s3.ino -- a JRA horse-racing fanfare
// voiced as three detuned sawtooth oscillators through an ADSR, with an
// explicit per-note gate/articulation schedule). The original sketch
// outputs I2S to a MAX98357A on an ESP32-S3; built with -tags cores3 this
// plays over I2S too, through the M5Stack CoreS3's own speaker amp.
//
// It's a close, function-for-function port: same three
// detuned Oscils, same ADSR configuration, same per-note gate-percent
// articulation state machine, same melody data (transcribed verbatim, not
// re-estimated).
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/TinyGo_Extras/fanfare_synth
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/envelope"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 256 // matches the original sketch's CONTROL_RATE
)

// note mirrors the original sketch's Note struct: a frequency, how long
// until the next melody event, and what percentage of that duration the
// note is held before NoteOff() starts the release phase.
type note struct {
	freq        float32
	durationMs  uint16
	gatePercent uint8
}

const restFreq float32 = 0

const (
	noteG4 float32 = 392.00

	noteC5 float32 = 523.25
	noteD5 float32 = 587.33
	noteE5 float32 = 659.25
	noteF5 float32 = 698.46
	noteG5 float32 = 783.99
	noteA5 float32 = 880.00
)

// Slight detuning makes the three oscillators sound like multiple brass
// instruments playing together.
const detuneRatio float32 = 0.0022

// A relatively fast envelope produces a clear brass attack. CONTROL_RATE
// is 256Hz, so extremely short ADSR times aren't useful; ~15-25ms gives a
// reliable attack at this rate.
const (
	attackMs  uint32 = 20
	decayMs   uint32 = 65
	releaseMs uint32 = 30

	attackLevel  uint8 = 255
	sustainLevel uint8 = 185
)

// Kept conservative because three sawtooth oscillators contain many
// harmonics and can get quite loud.
const masterVolume int32 = 170

// melody: the JRA Kanto GI fanfare, voiced with explicit re-articulation
// for repeated pitches (rather than treating them as one held note) --
// the opening is "Paa ... Pa-Pa-Paa ... Pa-Pa-Paa", not "Paaaaaaaa". Timing
// is based on attacks/pitch changes observed in a reference video, then
// lightly quantized for natural-sounding playback. Transcribed verbatim
// from the original .ino, not re-derived.
var melody = []note{
	// Opening D5 fanfare articulation.
	{noteD5, 820, 92},  // Paa
	{restFreq, 120, 0}, // small breath before the repeated figure
	{noteD5, 145, 58},  // Pa
	{noteD5, 145, 58},  // Pa
	{noteD5, 650, 92},  // Paa
	{restFreq, 110, 0}, // small breath
	{noteD5, 145, 58},  // Pa
	{noteD5, 145, 58},  // Pa
	{noteD5, 650, 92},  // Paa

	{noteF5, 1320, 94},

	// E5 C5 G4 -> F5: quick descending figure.
	{noteE5, 300, 76},
	{noteC5, 165, 68},
	{noteG4, 165, 68},
	{noteF5, 1320, 94},

	// E5 C5 G4 -> D5.
	{noteE5, 340, 76},
	{noteC5, 165, 68},
	{noteG4, 165, 68},
	{noteD5, 1110, 92},

	// G4 D5 F5.
	{noteG4, 520, 82},
	{noteD5, 520, 82},
	{noteF5, 520, 82},

	// E5 C5 G4.
	{noteE5, 520, 82},
	{noteC5, 520, 82},
	{noteG4, 520, 82},

	// D5, with a short re-articulation.
	{noteD5, 850, 90},
	{noteD5, 520, 78},

	// F5 E5.
	{noteF5, 520, 82},
	{noteE5, 520, 82},

	// A5 F5.
	{noteA5, 520, 82},
	{noteF5, 520, 82},

	// Final G5, re-articulated instead of one static tone.
	{noteG5, 1500, 94}, // first strong G
	{noteG5, 500, 80},
	{noteG5, 280, 65},
	{noteG5, 1700, 95}, // final sustained note

	{restFreq, 1000, 0}, // end silence
}

// msToTicks converts a millisecond duration to a number of UpdateControl ticks.
func msToTicks(ms uint16) uint32 {
	ticks := uint32(ms) * controlRate / 1000
	if ticks == 0 {
		ticks = 1
	}
	return ticks
}

type sketch struct {
	osc1, osc2, osc3 *oscil.Oscil
	env              *envelope.ADSR

	currentNote    int
	currentTicks   uint32
	remainingTicks uint32
	gateTicks      uint32

	playing      bool
	noteReleased bool
}

func newSketch() *sketch {
	s := &sketch{
		osc1:         oscil.New(tables.Saw2048, audioRate),
		osc2:         oscil.New(tables.Saw2048, audioRate),
		osc3:         oscil.New(tables.Saw2048, audioRate),
		env:          envelope.NewADSR(controlRate, audioRate),
		playing:      true,
		noteReleased: true,
	}

	s.env.SetLevels(attackLevel, sustainLevel, sustainLevel, 0)
	// Sustain is deliberately very long because the melody scheduler
	// explicitly calls NoteOff() according to each note's gate.
	s.env.SetTimes(attackMs, decayMs, 10000, releaseMs)

	return s
}

// setOscillatorFrequency sets all three oscillators, two of them slightly
// detuned around freq.
func (s *sketch) setOscillatorFrequency(freq float32) {
	s.osc1.SetFreqFloat(freq)
	s.osc2.SetFreqFloat(freq * (1 + detuneRatio))
	s.osc3.SetFreqFloat(freq * (1 - detuneRatio))
}

// startNote begins the melody event at index.
func (s *sketch) startNote(index int) {
	n := melody[index]

	s.currentTicks = msToTicks(n.durationMs)
	s.remainingTicks = s.currentTicks
	s.noteReleased = false

	if n.freq == restFreq {
		s.env.NoteOff()
		s.gateTicks = 0
		s.noteReleased = true
		return
	}

	s.gateTicks = s.currentTicks * uint32(n.gatePercent) / 100
	if s.gateTicks == 0 {
		s.gateTicks = 1
	}

	s.setOscillatorFrequency(n.freq)
	// Reset the envelope to zero on every note event: without this,
	// consecutive same-pitch notes could blend together instead of
	// sounding like separate attacks.
	s.env.NoteOnReset()
}

func (s *sketch) UpdateControl() {
	s.env.Update()

	if !s.playing {
		return
	}

	if s.remainingTicks > 0 {
		s.remainingTicks--

		if melody[s.currentNote].freq == restFreq {
			return // rests don't need gate processing
		}

		elapsed := s.currentTicks - s.remainingTicks
		if !s.noteReleased && elapsed >= s.gateTicks {
			s.env.NoteOff() // begin the release phase when the gate closes
			s.noteReleased = true
		}
		return
	}

	// Move to the next melody event.
	s.currentNote++
	if s.currentNote >= len(melody) {
		s.env.NoteOff()
		s.playing = false
		return
	}
	s.startNote(s.currentNote)
}

func (s *sketch) UpdateAudio() audioout.Mono {
	// Keep processing the release tail even after playback ends.
	if !s.playing && !s.env.Playing() {
		return audioout.From16Bit(0)
	}
	if melody[s.currentNote].freq == restFreq {
		return audioout.From16Bit(0)
	}

	// Three slightly detuned sawtooth waves; center oscillator weighted
	// higher (osc1 50%, osc2/osc3 25% each).
	s1 := int32(s.osc1.Next())
	s2 := int32(s.osc2.Next())
	s3 := int32(s.osc3.Next())
	mixed := (s1*2 + s2 + s3) / 4

	env := int32(s.env.Next()) // 0-255
	output := mixed * env
	output = (output * masterVolume) >> 8

	return audioout.From16Bit(output)
}

func main() {
	s := newSketch()

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	s.startNote(0)

	for {
		engine.AudioHook()
	}
}
