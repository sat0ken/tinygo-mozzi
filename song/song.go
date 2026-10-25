// Package song is a tiny monophonic melody sequencer, built on top of
// mozzi/oscil and mozzi/midi. It isn't part of upstream Mozzi; it exists
// to make "play this piece of music" sketches easy to write, and
// especially easy to correct against a real score: melodies are written
// as MIDI note numbers, and this package's note-name constants (see
// notes.go: C4, Fs5, ...) let you write them exactly the way they're
// named on a staff, rather than looking up numbers by hand.
package song

import (
	"mozzi/midi"
	"mozzi/oscil"
)

// Note is a MIDI note number.
type Note = uint8

// Rest is a sentinel Note meaning "silence" for an Event's duration.
// (MIDI note 0 has no valid pitch of its own anyway -- see mozzi/midi's
// note on that -- so it's a safe, zero-value-friendly choice here.)
const Rest Note = 0

// Event is one note (or Rest) and how long it lasts, in beats (quarter
// notes, relative to whatever tempo you give NewPlayer). A dotted note is
// 1.5x its plain duration; a note inside a bracketed "3" (triplet) gets
// 1/3 of that bracket's total beat value.
type Event struct {
	Note  Note
	Beats float32
}

// Player steps through a slice of Events, driving an Oscil's frequency.
type Player struct {
	osc    *oscil.Oscil
	events []Event
	loop   bool

	ticksPerBeat float32
	ticksLeft    float32
	index        int
	playing      bool
}

// NewPlayer creates a Player driving osc. controlRateHz must be the rate
// you'll call Update() at (normally your sketch's control rate, passed to
// engine.Start); bpm is tempo in quarter notes per minute (a score's
// "♩=120" marking means bpm=120). If loop is true, the melody restarts
// from the beginning when it finishes; if false, it plays once and then
// Playing() reports false forever.
func NewPlayer(osc *oscil.Oscil, controlRateHz uint32, bpm float32, events []Event, loop bool) *Player {
	p := &Player{
		osc:          osc,
		events:       events,
		loop:         loop,
		ticksPerBeat: float32(controlRateHz) * 60 / bpm,
	}
	if len(events) > 0 {
		p.startEvent(0)
	}
	return p
}

func (p *Player) startEvent(i int) {
	p.index = i
	e := p.events[i]
	p.ticksLeft = p.ticksPerBeat * e.Beats
	p.playing = e.Note != Rest
	if p.playing {
		p.osc.SetFreq(midi.MtoFWhole(e.Note))
	}
}

// Update advances the player by one control-rate tick. Call this from
// UpdateControl().
func (p *Player) Update() {
	if len(p.events) == 0 {
		return
	}
	p.ticksLeft--
	if p.ticksLeft > 0 {
		return
	}
	next := p.index + 1
	if next >= len(p.events) {
		if !p.loop {
			p.playing = false
			return
		}
		next = 0
	}
	p.startEvent(next)
}

// Playing reports whether a note (as opposed to a Rest, or the melody
// having finished with loop=false) is currently sounding. Check this in
// UpdateAudio() to silence output during rests:
//
//	func (s *sketch) UpdateAudio() audioout.Mono {
//	    if !s.player.Playing() {
//	        return audioout.From8Bit(0)
//	    }
//	    return audioout.From8Bit(int16(s.osc.Next()))
//	}
func (p *Player) Playing() bool { return p.playing }
