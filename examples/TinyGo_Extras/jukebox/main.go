// Command jukebox plays a few tunes (see songs.go) in two parts -- a
// melody on a triangle wave and an accompaniment on a sine -- each note
// shaped by its own ADSR envelope, so they sound plucked rather than
// switched on and off like the single-voice mozzi/song examples. The tunes
// play in turn; the board's button (mozzi/board.Button: SW1 on the TinyGo
// Conf 2026 badge) skips to the next one. The title of each is printed over
// USB serial as it starts.
//
// It isn't a port of an upstream example: it's built from mozzi's oscil,
// envelope and midi packages, with mozzi/song's note names and Event type
// for the scores.
//
// Build & flash, e.g. for the TinyGo Conf 2026 badge (see mozzi/board for
// the other boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags tinygoconf2026badge -port /dev/ttyACM0 ./examples/TinyGo_Extras/jukebox
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/envelope"
	"mozzi/fixmath"
	"mozzi/midi"
	"mozzi/oscil"
	"mozzi/song"
	"mozzi/tables"
)

const (
	audioRate = 16384
	// controlRate is higher than mozzi's usual 64Hz so the envelopes' attack
	// can be a few milliseconds: at 64Hz a control step is 16ms, and an
	// attack shorter than one step clicks.
	controlRate = 256

	pauseBetweenTunes = controlRate // one second
)

// voice plays one part of a tune: an oscillator, its envelope, and where it
// is in the score.
type voice struct {
	osc *oscil.Oscil
	env *envelope.ADSR

	attackMs, decayMs, releaseMs uint32
	gate                         float32 // how much of each note's length it's held for

	events    []song.Event
	index     int
	ticksLeft float32
}

func newVoice(table []int8, sustain uint8, attackMs, decayMs, releaseMs uint32, gate float32) *voice {
	v := &voice{
		osc:      oscil.New(table, audioRate),
		env:      envelope.NewADSR(controlRate, audioRate),
		attackMs: attackMs, decayMs: decayMs, releaseMs: releaseMs,
		gate: gate,
	}
	v.env.SetADLevels(255, sustain)
	return v
}

func (v *voice) start(events []song.Event) {
	v.events = events
	v.index = -1
	v.ticksLeft = 1 // the first tick starts the first note
}

func (v *voice) done() bool { return v.index >= len(v.events) }

// tick advances the voice by one control tick, starting the next note when
// the current one's time is up.
func (v *voice) tick(ticksPerBeat, msPerBeat float32) {
	v.env.Update()
	if v.done() {
		return
	}
	v.ticksLeft--
	if v.ticksLeft > 0 {
		return
	}
	v.index++
	if v.done() {
		return
	}
	e := v.events[v.index]
	v.ticksLeft += ticksPerBeat * e.Beats // +=, not =, so rounding never accumulates
	if e.Note == song.Rest {
		return // the previous note just carries on releasing
	}
	v.osc.SetFreqQ16n16(midi.Q16n16MtoF(fixmath.Q16n16(e.Note) << 16))
	held := uint32(e.Beats * msPerBeat * v.gate)
	sustain := uint32(0)
	if held > v.attackMs+v.decayMs {
		sustain = held - v.attackMs - v.decayMs
	}
	v.env.SetTimes(v.attackMs, v.decayMs, sustain, v.releaseMs)
	v.env.NoteOn()
}

type sketch struct {
	melody, accomp *voice
	tune           int
	ticksPerBeat   float32
	msPerBeat      float32
	pause          int // control ticks left before the next tune starts

	buttonWas   bool
	buttonTicks int // control ticks the button has been in its current state
}

func (s *sketch) play(i int) {
	s.tune = i % len(tunes)
	t := tunes[s.tune]
	s.ticksPerBeat = controlRate * 60 / t.bpm
	s.msPerBeat = 60000 / t.bpm
	s.melody.start(t.melody)
	s.accomp.start(t.accomp)
	println("now playing:", t.title)
}

func (s *sketch) UpdateControl() {
	// Skip to the next tune on a button press, once it has been down for
	// 5 ticks (~20ms), so contact bounce doesn't skip twice.
	pressed := board.ButtonPressed()
	if pressed != s.buttonWas {
		s.buttonWas, s.buttonTicks = pressed, 0
	} else if s.buttonTicks < 1000 {
		s.buttonTicks++
	}
	if pressed && s.buttonTicks == 5 {
		s.play(s.tune + 1)
	}

	s.melody.tick(s.ticksPerBeat, s.msPerBeat)
	s.accomp.tick(s.ticksPerBeat, s.msPerBeat)
	if s.melody.done() && s.accomp.done() {
		if s.pause == 0 {
			s.pause = pauseBetweenTunes
		} else if s.pause--; s.pause == 0 {
			s.play(s.tune + 1)
		}
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	m := int32(s.melody.osc.Next()) * int32(s.melody.env.Next())
	a := int32(s.accomp.osc.Next()) * int32(s.accomp.env.Next())
	// Each product is 16 bit; weighted 3:1 they still can't exceed 16 bits.
	return audioout.FromNBit(16, m*3/4+a/4)
}

func main() {
	board.ConfigureButton()
	s := &sketch{
		// melody: a quick, plucked attack that decays to a held level
		melody: newVoice(tables.Triangle2048, 140, 8, 250, 150, 0.9),
		// accompaniment: softer, fading out over each note
		accomp: newVoice(tables.Sin2048, 90, 8, 200, 120, 0.8),
	}
	s.play(0)

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
