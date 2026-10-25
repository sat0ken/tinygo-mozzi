//go:build rp2040 && !rp2040i2s

package board

import (
	"machine"

	"mozzi/backend/rp2040pwm"
	"mozzi/engine"
)

const (
	audioPin  = machine.GPIO15
	carrierHz = 250_000

	A0     = machine.ADC0  // GPIO26
	Button = machine.GPIO4 // a button to 3.3V

	buttonActiveLow = false
	defaultVolume   = 255 // no effect: PWM output has no volume control
)

// NewOutput sets up PWM audio out of GPIO15 (put an RC filter between it
// and your amp; see backend/rp2040pwm).
func NewOutput(audioRate uint32) (engine.Backend, error) {
	o, err := rp2040pwm.New(audioPin, carrierHz)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func applyVolume(uint8) {}
