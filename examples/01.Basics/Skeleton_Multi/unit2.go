package main

import "mozzi/audioout"

func (s *sketch) UpdateAudio() audioout.Mono {
	return audioout.From8Bit(0) // just a dummy
}
