// Package samplehuffman is a port of Mozzi's SampleHuffman.h: playback of
// a recorded sound stored compressed, as Huffman-coded differences between
// successive 8 bit samples (as made by Mozzi's extras/python/audio2huff.py;
// see mozzi/samples for the converted sounds).
package samplehuffman

// SampleHuffman plays one Huffman-compressed sample, one value per Next
// call (so at the rate it was recorded at, if Next is called from
// UpdateAudio at that audio rate).
type SampleHuffman struct {
	soundData     []uint8 // the Huffman-coded bit stream, MSB first
	huffman       []int16 // the decoding tree
	soundDataBits uint32  // number of valid bits in soundData

	datapos uint32 // current bit position
	current int16  // current amplitude
	looping bool
	bt      uint8 // the byte of soundData currently being read
}

// New creates a SampleHuffman from a sample's three parts: its sound data,
// Huffman tree and sound data bit count (upstream's NAME_SOUNDDATA,
// NAME_HUFFMAN and NAME_SOUNDDATA_BITS). Like upstream's (global, hence
// zero-initialized) objects, it starts out playing from the beginning;
// Start restarts it.
func New(soundData []uint8, huffman []int16, soundDataBits uint32) *SampleHuffman {
	return &SampleHuffman{soundData: soundData, huffman: huffman, soundDataBits: soundDataBits}
}

// Next returns the next sample value, or 0 once the end is reached (unless
// looping is on).
func (s *SampleHuffman) Next() int16 {
	if s.datapos >= s.soundDataBits {
		if !s.looping {
			return 0
		}
		s.datapos = 0
	}
	s.current += s.decode() // add the decoded difference
	return s.current
}

// SetLoopingOn makes the sample repeat from the start when it reaches the end.
func (s *SampleHuffman) SetLoopingOn() { s.looping = true }

// SetLoopingOff makes the sample stop (Next returns 0) when it reaches the end.
func (s *SampleHuffman) SetLoopingOff() { s.looping = false }

// Start (re)starts playback from the beginning.
func (s *SampleHuffman) Start() {
	s.current = 0
	s.datapos = 0
	s.bt = 0
}

func (s *SampleHuffman) getBit() bool {
	b := s.datapos & 7
	if b == 0 {
		s.bt = s.soundData[s.datapos>>3]
	}
	return (s.bt>>(7-b))&1 != 0
}

// decode walks the Huffman tree, one bit of soundData per branch, to the
// next difference value. Same layout as upstream: each node is either
// {0, value} (a leaf) or {offset, ...} (branch: on a 1 bit, skip ahead
// offset+1 entries, or 2 if offset is 0).
func (s *SampleHuffman) decode() int16 {
	i := 0
	for {
		if s.getBit() {
			offs := int(s.huffman[i])
			if offs != 0 {
				i += offs + 1
			} else {
				i += 2
			}
		}
		s.datapos++
		leaf := s.huffman[i] == 0
		i++
		if leaf {
			return s.huffman[i]
		}
	}
}
