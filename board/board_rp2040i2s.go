//go:build rp2040 && rp2040i2s

package board

import (
	"machine"

	pio "github.com/tinygo-org/pio/rp2-pio"

	"mozzi/backend/rp2040i2s"
	"mozzi/engine"
)

const (
	i2sDataPin  = machine.GPIO2
	i2sClockPin = machine.GPIO3 // BCLK; LRC is automatically i2sClockPin+1 (GPIO4)

	A0     = machine.ADC0  // GPIO26
	Button = machine.GPIO5 // a button to 3.3V

	buttonActiveLow = false
	defaultVolume   = 255 // no effect: this backend has no volume control
)

// NewOutput sets up I2S out to an external DAC/amp, e.g. a MAX98357A: DIN on
// GPIO2, BCLK on GPIO3, LRC on GPIO4 (see backend/rp2040i2s).
func NewOutput(audioRate uint32) (engine.Backend, error) {
	o, err := rp2040i2s.New(pio.PIO0, i2sDataPin, i2sClockPin)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func applyVolume(uint8) {}
