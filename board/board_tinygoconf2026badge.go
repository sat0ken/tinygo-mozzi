//go:build tinygoconf2026badge

package board

import (
	"machine"

	"mozzi/backend/esp32s3i2s"
	"mozzi/engine"
)

// The TinyGo Conf 2026 badge: an ESP32-S3 DevKit on a board with a
// MAX98357A I2S amp and speaker, a joystick and two switches
// (https://github.com/tinygo-keeb/tinygo-conf-2026-badge; pins from its
// firmware/badge package).
const (
	i2sBCLK = machine.GPIO45
	i2sLRC  = machine.GPIO21
	i2sDIN  = machine.GPIO47

	A0     = machine.GPIO7  // the joystick's X axis (ADC1 channel 6)
	Button = machine.GPIO13 // SW1, which pulls the pin to GND

	buttonActiveLow = true
	// defaultVolume is the level the badge's own firmware plays at: its
	// MAX98357A is set to 12dB gain and distorts when driven much harder.
	defaultVolume = 32
)

var out *esp32s3i2s.Output

// NewOutput sets up I2S out to the badge's MAX98357A (no amp setup needed).
func NewOutput(audioRate uint32) (engine.Backend, error) {
	o, err := esp32s3i2s.New(esp32s3i2s.Config{BCLK: i2sBCLK, WS: i2sLRC, DOUT: i2sDIN})
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
