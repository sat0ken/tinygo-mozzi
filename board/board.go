// Package board picks the audio output, and a couple of input pins, for the
// board a sketch is built for, so one sketch -- everything under examples/
// -- builds for every board this port supports, the way an upstream Mozzi
// sketch builds for any board Mozzi supports. The board is chosen at build
// time, from the TinyGo target plus, where one target covers more than one
// board, a build tag:
//
//	Board                                 Build flags                                     Audio out
//	M5Stack CoreS3 / CoreS3 SE            -target=esp32s3-generic -tags cores3            built-in speaker (board/cores3, backend/esp32s3i2s)
//	any ESP32-S3, PWM + RC filter         -target=esp32s3-generic -tags esp32s3pwm        GPIO4 (backend/esp32s3pwm)
//	Raspberry Pi Pico / any RP2040, PWM   -target=pico (or another RP2040 target)         GPIO15 (backend/rp2040pwm)
//	RP2040 + I2S DAC/amp (e.g. MAX98357A) -target=waveshare-rp2040-zero -tags rp2040i2s   DIN=GPIO2, BCLK=GPIO3, LRC=GPIO4 (backend/rp2040i2s)
//	TinyGo Conf 2026 badge (ESP32-S3      -target=esp32s3-generic -tags tinygoconf2026badge  its MAX98357A: BCLK=GPIO45, LRC=GPIO21, DIN=GPIO47 (backend/esp32s3i2s)
//	  DevKit + MAX98357A)
//
// Building for an ESP32-S3 without one of its tags is a compile error rather
// than a guess: the set-ups use different pins, and the PWM one's GPIO4 is
// the CoreS3's microSD card chip-select line.
//
// Besides NewOutput, each board defines A0, the pin to use where an
// upstream sketch reads Arduino analog input 0, and Button, a pin for a
// push button, read with ConfigureButton/ButtonPressed (which know whether
// the board's button pulls the pin high or low):
//
//	Board                 A0                          Button
//	CoreS3                GPIO8 (Port B)              GPIO9 (Port B), to 3.3V
//	ESP32-S3 PWM          GPIO1                       GPIO5, to 3.3V
//	RP2040 PWM            GPIO26 (ADC0)               GPIO4, to 3.3V
//	RP2040 I2S            GPIO26 (ADC0)               GPIO5, to 3.3V
//	TinyGo Conf 2026      GPIO7 (joystick X axis)     GPIO13 (SW1, to GND)
//
// Volume: on boards whose output can scale it without losing resolution
// (the I2S ones: CoreS3 and the TinyGo Conf 2026 badge), SetVolume sets the
// output level, 0-255; the default depends on the board (255 on the CoreS3,
// whose amp is attenuated instead; 32 on the badge, whose MAX98357A at 12dB
// gain distorts above that, as its own firmware notes). It can also be set
// at build time, without editing the sketch, with e.g.
// -ldflags="-X mozzi/board.volume=128". Elsewhere it does nothing.
package board

// volume is the initial output level, as a decimal string so it can be
// overridden at link time: -ldflags="-X mozzi/board.volume=128". Empty
// means the board's default.
var volume = ""

var level = parseVolume(volume)

// SetVolume sets the output level, from 0 (silent) to 255 (full scale), on
// boards that support it (see the package doc for their defaults); 128 is
// about half amplitude (-6dB). It can be called before or after NewOutput.
func SetVolume(v uint8) {
	level = v
	applyVolume(v)
}

// Volume returns the output level set by SetVolume or at build time.
func Volume() uint8 { return level }

func parseVolume(s string) uint8 {
	if s == "" {
		return defaultVolume
	}
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVolume
		}
		v = v*10 + int(c-'0')
		if v > 255 {
			return 255
		}
	}
	return uint8(v)
}
