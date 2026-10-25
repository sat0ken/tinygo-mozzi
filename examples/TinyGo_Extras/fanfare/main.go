// Command fanfare is a melody-player skeleton for "東京・中山G1競争
// 競馬ファンファーレ" (a JRA horse-racing fanfare), built on mozzi/song.
//
// The `melody` slice below is transcribed by directly analyzing a
// recording of the piece (ffmpeg to extract audio, then a small
// autocorrelation-based pitch tracker + note segmenter) -- both pitches
// and durations are measured, not estimated, for effectively the whole
// piece (the note-by-note-timed version of the analysis in the chat this
// was derived in). It only covers the lead trumpet line, not the other
// parts (harmony, drums, ...).
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/TinyGo_Extras/fanfare
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/oscil"
	"mozzi/song"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 512 // higher than this port's other examples, for tighter note-change timing
	bpm         = 112 // measured tempo (back down from the earlier 150 -- see melody's comment: that
	// speedup was compensating for underestimated note durations, not an actually-slow tempo)

)

// melody is the lead trumpet line. Every pitch and duration below comes
// from measured note onset/offset times in the recording (converted to
// beats at the bpm above), not estimation -- see this file's package doc.
// Three things worth knowing about how it was derived:
//   - Two short beeps right at the start of the recording (~0.33s and
//     ~0.87s, unrelated to the tune -- probably a recording-start chime)
//     are not included.
//   - The brief silences between phrases (breath gaps) are included as
//     explicit song.Rest events, for a more natural feel than butting
//     every phrase up against the next.
//   - The opening "D5" is not one sustained tone: the energy envelope
//     (independent of pitch, which stays on D5 throughout) has 6 clear
//     near-silent dips in it, i.e. it's re-tongued repeatedly -- a
//     long-short-short cell played twice, into a short pickup into F5.
//     Simple pitch-only segmentation missed this entirely (same pitch
//     before and after each dip). The song.Rest gaps this needs to
//     actually re-articulate on this synth (two back-to-back same-pitch
//     Events would otherwise just sound like one held note -- Player
//     doesn't re-gate same-pitch transitions) split each repeat's
//     measured cycle time as ~85% note / ~15% silence; that split, but
//     not each repeat's start time, is estimated. F5 right after it shows
//     similar but much shallower wobble in its energy -- closer to
//     vibrato than distinct re-tonguing -- so it's kept as one note; flag
//     it if that still sounds off once you hear it.
var melody = []song.Event{
	{song.D5, 1.251}, {song.Rest, 0.221},
	{song.D5, 0.238}, {song.Rest, 0.042},
	{song.D5, 0.206}, {song.Rest, 0.036},
	{song.D5, 1.251}, {song.Rest, 0.221},
	{song.D5, 0.221}, {song.Rest, 0.039},
	{song.D5, 0.222}, {song.Rest, 0.039},
	{song.D5, 0.112}, // short pickup straight into F5

	{song.F5, 2.905},

	{song.E5, 0.355}, {song.C5, 0.355}, {song.G4, 0.259}, // triplet
	{song.F5, 2.069},
	{song.Rest, 0.764}, // breath

	{song.E5, 0.353}, {song.C5, 0.392}, {song.G4, 0.299}, // triplet
	{song.D5, 2.235},
	{song.Rest, 0.689}, // breath

	{song.G4, 0.857},
	{song.D5, 0.969},
	{song.F5, 0.932},
	{song.E5, 0.747},
	{song.C5, 1.247},
	{song.G4, 0.913},
	{song.D5, 1.509},
	{song.Rest, 0.521}, // breath

	{song.D5, 0.653},
	{song.Rest, 0.297}, // breath
	{song.F5, 0.894},
	{song.E5, 1.006},
	{song.A5, 0.932},
	{song.F5, 0.745},
	{song.Rest, 0.317}, // breath

	{song.G5, 10.317}, // long final held note (a cymbal roll/crash sounds under it in the recording)
}

type sketch struct {
	osc    *oscil.Oscil
	player *song.Player
}

func (s *sketch) UpdateControl() {
	s.player.Update()
}

func (s *sketch) UpdateAudio() audioout.Mono {
	if !s.player.Playing() {
		return audioout.From8Bit(0)
	}
	return audioout.From8Bit(int16(s.osc.Next()))
}

func main() {
	osc := oscil.New(tables.Sin2048, audioRate)
	s := &sketch{
		osc:    osc,
		player: song.NewPlayer(osc, controlRate, bpm, melody, true), // loop
	}

	out, err := board.NewOutput(audioRate)
	if err != nil {
		panic(err)
	}

	engine.Start(s, audioRate, controlRate, out)
	for {
		engine.AudioHook()
	}
}
