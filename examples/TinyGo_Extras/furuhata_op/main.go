// Command furuhata_op is a melody-player skeleton for the "古畑任三郎"
// (Furuhata Ninzaburō) opening theme (作曲: 本間勇輔), built on mozzi/song.
//
// Source material: a PDF transcription (Furuhata_Opening_Theme_C_doremi.pdf)
// itself produced elsewhere from a video showing on-screen sheet music +
// solfège. That PDF's text layer is a notation-software export, and reads
// back badly scrambled -- one representative note name per measure, not a
// full, ordered note list, so it can't be mechanically parsed into a
// melody. What IS clear and repeated consistently across the PDF is the
// Intro's driving two-note ostinato, alternating F#5 and C#5 (a perfect
// 5th apart) -- implemented below with reasonable confidence.
//
// The `melody` slice covers ONLY that Intro ostinato. Sections A through F
// (labelled in the PDF as メインモチーフ/展開/応答フレーズ/再現/クライマ
// ックス/エンディング) are not included -- their note-by-note content
// wasn't reliably extractable from the PDF text, and guessing would just
// produce a melody that confidently plays the wrong notes. If you have the
// source video (the one the PDF's sheet-music overlay came from) as a
// local file, the same ffmpeg-plus-pitch-tracking approach used for
// examples/TinyGo_Extras/fanfare's melody works far better than reading a PDF/photo of
// notation -- happy to redo this the same way if you can point at that file.
//
// Rhythm caveat: even the Intro's *pitches* (F#5/C#5) are solid, but the
// exact rhythm shown per measure in the PDF's rendered image wasn't fully
// legible either. What's below is 8 straight eighth notes per 4/4 measure
// (F#-C#-F#-C#-F#-C#-F#-C#) -- a reasonable, simple ostinato consistent
// with the alternation the PDF shows, not a confirmed transcription of the
// notated rhythm. Retime individual Beats values if it doesn't match once
// you hear it against the source.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/TinyGo_Extras/furuhata_op
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
	bpm         = 138 // the PDF's "q = 138" tempo marking

)

// melody is the Intro's F#5/C#5 ostinato, repeated for 4 measures of 4/4
// (matching the PDF's "Intro / リズム提示" section, measures 1-4). See the
// package doc above for what's solid here (the pitches) versus guessed
// (the straight-eighth-notes rhythm).
var melody = []song.Event{
	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5},
	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5}, // measure 1

	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5},
	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5}, // measure 2

	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5},
	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5}, // measure 3

	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5},
	{song.Fs5, 0.5}, {song.Cs5, 0.5}, {song.Fs5, 0.5}, {song.Cs5, 0.5}, // measure 4
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
