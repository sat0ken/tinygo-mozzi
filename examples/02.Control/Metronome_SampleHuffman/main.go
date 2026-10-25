// Command Metronome_SampleHuffman is a port of Mozzi's
// examples/02.Control/Metronome_SampleHuffman: a Metronome triggering one of
// five Huffman-compressed thumb piano samples at random, every 800ms, with
// the Metronome periodically stopped and restarted.
//
// The samples are upstream's samples/thumbpiano_huffman, converted by
// tools/genhuffman into mozzi/samples/thumbpianohuffman.
//
// Build & flash, e.g. for an M5Stack CoreS3 (see mozzi/board for the other
// boards and their pins):
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Metronome_SampleHuffman
package main

import (
	"mozzi/audioout"
	"mozzi/board"
	"mozzi/engine"
	"mozzi/metronome"
	"mozzi/rand"
	"mozzi/samplehuffman"
	thumb "mozzi/samples/thumbpianohuffman"
)

const (
	audioRate   = 16384 // the samples' recorded rate
	controlRate = 64    // upstream's default

	numSamples = 5
)

type sketch struct {
	thumbs  [numSamples]*samplehuffman.SampleHuffman
	kMetro  *metronome.Metronome
	counter uint32
}

func (s *sketch) UpdateControl() {
	s.counter++
	if s.counter == 177 {
		s.kMetro.Stop()
	}
	if s.counter == 203 {
		s.kMetro.Start()
		s.counter = 0
	}
	if s.kMetro.Ready() {
		s.thumbs[rand.Int16Max(numSamples)].Start() // upstream's rand(NUM_SAMPLES): a char, promoted to the int overload
	}
}

func (s *sketch) UpdateAudio() audioout.Mono {
	var asig int32
	for _, t := range s.thumbs {
		asig += int32(t.Next())
	}
	return audioout.From8Bit(int16(asig))
}

func main() {
	s := &sketch{
		thumbs: [numSamples]*samplehuffman.SampleHuffman{
			samplehuffman.New(thumb.Thumb0SoundData, thumb.Thumb0Huffman, thumb.Thumb0SoundDataBits),
			samplehuffman.New(thumb.Thumb1SoundData, thumb.Thumb1Huffman, thumb.Thumb1SoundDataBits),
			samplehuffman.New(thumb.Thumb2SoundData, thumb.Thumb2Huffman, thumb.Thumb2SoundDataBits),
			samplehuffman.New(thumb.Thumb3SoundData, thumb.Thumb3Huffman, thumb.Thumb3SoundDataBits),
			samplehuffman.New(thumb.Thumb4SoundData, thumb.Thumb4Huffman, thumb.Thumb4SoundDataBits),
		},
		kMetro: metronome.New(800), // enough delay so samples don't overlap, because the load would be too much
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
