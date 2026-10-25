//go:build esp32s3pwm

package board

import (
	"machine"

	"mozzi/backend/esp32s3pwm"
	"mozzi/engine"
)

const (
	audioPin  = machine.GPIO4
	carrierHz = 250_000

	A0     = machine.GPIO1 // ADC1 channel 0
	Button = machine.GPIO5 // a button to 3.3V

	buttonActiveLow = false
	defaultVolume   = 255 // no effect: PWM output has no volume control
)

// NewOutput sets up PWM audio out of GPIO4 (put an RC filter between it and
// your amp; see backend/esp32s3pwm).
func NewOutput(audioRate uint32) (engine.Backend, error) {
	o, err := esp32s3pwm.New(audioPin, carrierHz)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func applyVolume(uint8) {}
