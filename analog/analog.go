// Package analog is a port of mozziAnalogRead, from Mozzi's mozzi_analog.h.
//
// Upstream reads analog inputs asynchronously on AVR (and a few other
// chips), so that a read never holds up audio. On ESP32, RP2040 and most
// other 32 bit boards upstream instead falls back to a plain, blocking
// analogRead(); this port does the same, with TinyGo's machine.ADC. A
// one-shot conversion takes a few microseconds on these chips, so reading
// a handful of inputs from UpdateControl is fine.
package analog

import "machine"

var (
	adcReady   bool
	configured [256]bool // by pin number
)

// Read returns a reading of the analog input on pin, scaled to bits of
// resolution -- Read(pin, 10) is upstream's mozziAnalogRead<10>(pin), in
// the range 0-1023. bits must be between 1 and 16.
func Read(pin machine.Pin, bits uint8) uint16 {
	if !adcReady {
		machine.InitADC()
		adcReady = true
	}
	adc := machine.ADC{Pin: pin}
	if !configured[pin] {
		adc.Configure(machine.ADCConfig{})
		configured[pin] = true
	}
	return adc.Get() >> (16 - bits) // machine.ADC.Get is scaled to 16 bits
}
