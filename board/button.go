//go:build rp2040 || esp32s3

package board

import "machine"

// ConfigureButton sets Button up as an input, with the pull-up or pull-down
// the board's button needs (see the package doc).
func ConfigureButton() {
	mode := machine.PinInputPulldown
	if buttonActiveLow {
		mode = machine.PinInputPullup
	}
	Button.Configure(machine.PinConfig{Mode: mode})
}

// ButtonPressed reports whether Button is pressed. Call ConfigureButton first.
func ButtonPressed() bool { return Button.Get() != buttonActiveLow }
