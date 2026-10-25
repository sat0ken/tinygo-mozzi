//go:build cores3

package board

import (
	"machine"

	"mozzi/backend/esp32s3i2s"
	"mozzi/board/cores3"
	"mozzi/engine"
)

const (
	A0     = machine.GPIO8 // Port B, the pin next to 5V
	Button = machine.GPIO9 // Port B, the other signal pin; a button to 3.3V

	buttonActiveLow = false
	defaultVolume   = 255 // the amp itself is attenuated, see board/cores3
)

var out *esp32s3i2s.Output

// NewOutput sets up the CoreS3's built-in speaker for audioRate.
func NewOutput(audioRate uint32) (engine.Backend, error) {
	o, err := cores3.NewSpeaker(audioRate)
	if err != nil {
		return nil, err
	}
	o.SetVolume(level)
	out = o
	return o, nil
}

func applyVolume(v uint8) {
	if out != nil {
		out.SetVolume(v)
	}
}
