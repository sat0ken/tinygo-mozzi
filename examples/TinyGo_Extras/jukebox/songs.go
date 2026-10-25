package main

import . "mozzi/song"

// A tune: a melody and an accompaniment, each a list of notes in beats
// (quarter notes) that add up to the same total, played at bpm.
type tune struct {
	title          string
	bpm            float32
	melody, accomp []Event
}

// All three are in the public domain. The melodies are as written; the
// accompaniments to the first two are simple broken chords, and Für Elise's
// is its own left hand. Für Elise is an octave up from the score, to suit
// small speakers.
var tunes = []tune{
	{"Ode to Joy (Beethoven)", 120, odeToJoy, arpeggios(
		C, C, G, G, C, C, G, G, // bars 1-4, two chords to a bar
		C, C, G, G, C, C, G, C,
		G, C, G, C, G, G, C, G,
		C, C, G, G, C, C, G, C,
	)},
	{"Twinkle, Twinkle, Little Star", 100, twinkle, arpeggios(
		C, C, F, C, F, C, G, C,
		C, F, C, G, C, F, C, G,
		C, C, F, C, F, C, G, C,
	)},
	{"Für Elise (Beethoven), opening", 100, furEliseRight, furEliseLeft},
}

var odeToJoy = []Event{
	{E5, 1}, {E5, 1}, {F5, 1}, {G5, 1}, {G5, 1}, {F5, 1}, {E5, 1}, {D5, 1},
	{C5, 1}, {C5, 1}, {D5, 1}, {E5, 1}, {E5, 1.5}, {D5, 0.5}, {D5, 2},
	{E5, 1}, {E5, 1}, {F5, 1}, {G5, 1}, {G5, 1}, {F5, 1}, {E5, 1}, {D5, 1},
	{C5, 1}, {C5, 1}, {D5, 1}, {E5, 1}, {D5, 1.5}, {C5, 0.5}, {C5, 2},
	{D5, 1}, {D5, 1}, {E5, 1}, {C5, 1}, {D5, 1}, {E5, 0.5}, {F5, 0.5}, {E5, 1}, {C5, 1},
	{D5, 1}, {E5, 0.5}, {F5, 0.5}, {E5, 1}, {D5, 1}, {C5, 1}, {D5, 1}, {G4, 2},
	{E5, 1}, {E5, 1}, {F5, 1}, {G5, 1}, {G5, 1}, {F5, 1}, {E5, 1}, {D5, 1},
	{C5, 1}, {C5, 1}, {D5, 1}, {E5, 1}, {D5, 1.5}, {C5, 0.5}, {C5, 2},
}

var twinkle = []Event{
	{C5, 1}, {C5, 1}, {G5, 1}, {G5, 1}, {A5, 1}, {A5, 1}, {G5, 2},
	{F5, 1}, {F5, 1}, {E5, 1}, {E5, 1}, {D5, 1}, {D5, 1}, {C5, 2},
	{G5, 1}, {G5, 1}, {F5, 1}, {F5, 1}, {E5, 1}, {E5, 1}, {D5, 2},
	{G5, 1}, {G5, 1}, {F5, 1}, {F5, 1}, {E5, 1}, {E5, 1}, {D5, 2},
	{C5, 1}, {C5, 1}, {G5, 1}, {G5, 1}, {A5, 1}, {A5, 1}, {G5, 2},
	{F5, 1}, {F5, 1}, {E5, 1}, {E5, 1}, {D5, 1}, {D5, 1}, {C5, 2},
}

// Für Elise, bars 1-8 with the pickup, in 3/8: a sixteenth is a quarter beat.
var furEliseRight = []Event{
	{E6, .25}, {Ds6, .25}, // pickup
	{E6, .25}, {Ds6, .25}, {E6, .25}, {B5, .25}, {D6, .25}, {C6, .25},
	{A5, .5}, {Rest, .25}, {C5, .25}, {E5, .25}, {A5, .25},
	{B5, .5}, {Rest, .25}, {E5, .25}, {Gs5, .25}, {B5, .25},
	{C6, .5}, {Rest, .25}, {E5, .25}, {E6, .25}, {Ds6, .25},
	{E6, .25}, {Ds6, .25}, {E6, .25}, {B5, .25}, {D6, .25}, {C6, .25},
	{A5, .5}, {Rest, .25}, {C5, .25}, {E5, .25}, {A5, .25},
	{B5, .5}, {Rest, .25}, {E5, .25}, {C6, .25}, {B5, .25},
	{A5, 1.5},
}

var furEliseLeft = []Event{
	{Rest, .5}, {Rest, 1.5}, // pickup, bar 1
	{A3, .25}, {E4, .25}, {A4, .25}, {Rest, .75},
	{E3, .25}, {E4, .25}, {Gs4, .25}, {Rest, .75},
	{A3, .25}, {E4, .25}, {A4, .25}, {Rest, .75},
	{Rest, 1.5},
	{A3, .25}, {E4, .25}, {A4, .25}, {Rest, .75},
	{E3, .25}, {E4, .25}, {Gs4, .25}, {Rest, .75},
	{A3, .25}, {E4, .25}, {A4, .25}, {Rest, .75},
}

// Broken-chord shapes, lowest note first: root position C, and G and F with
// their thirds in the bass, so the accompaniment moves by small steps.
var (
	C = [4]Note{C4, G4, E4, G4}
	G = [4]Note{B3, G4, D4, G4}
	F = [4]Note{A3, F4, C4, F4}
)

// arpeggios turns a list of chords, two beats each, into eighth notes.
func arpeggios(chords ...[4]Note) []Event {
	events := make([]Event, 0, 4*len(chords))
	for _, c := range chords {
		for _, n := range c {
			events = append(events, Event{Note: n, Beats: 0.5})
		}
	}
	return events
}
