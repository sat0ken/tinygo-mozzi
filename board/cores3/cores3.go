// Package cores3 sets up an M5Stack CoreS3 (or CoreS3 SE) to play mozzi
// audio through its built-in speaker: NewSpeaker wakes up the board's
// AW88298 speaker amp over I2C and returns a mozzi/backend/esp32s3i2s
// Output wired to it, ready to pass to engine.Start:
//
//	out, err := cores3.NewSpeaker(audioRate)
//	if err != nil {
//		panic(err)
//	}
//	out.SetVolume(128) // optional: about half amplitude (-6dB); 255 is full scale
//	engine.Start(s, audioRate, controlRate, out)
//
// The AW88298 sits on the CoreS3's internal I2C bus (G12/G11), with its
// reset line on the AW9523B IO expander's P0_2, and gets its I2S from
// BCK=G34, WS=G33, DOUT=G13. The register values here follow M5Unified's
// _speaker_enabled_cb_cores3 (and M5GFX's AW9523B port setup). The amp's
// 1.8V supply is the AXP2101 PMIC's ALDO1, which this only reads: it's on
// by default, and M5Stack's own firmware keeps it on too.
package cores3

import (
	"machine"

	"mozzi/backend/esp32s3i2s"
)

// Amp levels. M5Unified runs the AW88298 at full gain (register 0x0C =
// 0x0064, VOL 0dB, no input attenuation), but it never sends the amp a
// full-scale signal; mozzi sketches routinely do (From8Bit(osc.Next()) is
// full scale), and that clips the amp: measured on a CoreS3 playing a
// full-scale 440Hz sine, its clipping status bit (SYSST bit 7) kept
// setting and its output stage kept stopping and restarting -- heard as
// popcorn-like crackling instead of a tone. Attenuating only at the amp's
// volume stage isn't enough (it comes after the internal processing that
// clips); attenuating the I2S input by 6dB (I2SCTRL.INPLEV) and the volume
// by 12dB ran clean, with volume (esp32s3i2s.Output.SetVolume /
// board.SetVolume) at 255. So that's the setting, making 255 the loudest
// level that doesn't clip.
const (
	// ampVolume is register 0x0C (HAGCCFG4): VOL in bits 15:8 -- bits 7:4
	// in -6dB steps, 3:0 in -0.5dB steps -- here -12dB, and the default
	// attack hold time (HOLDTH, 0x64) in bits 7:0.
	ampVolume = 0x2064
	// ampInputLevel is I2SCTRL.INPLEV (register 0x06 bit 13): -6dB at the
	// I2S input.
	ampInputLevel = 1 << 13
)

// Speaker I2S pins.
const (
	SpeakerBCLK = machine.GPIO34
	SpeakerWS   = machine.GPIO33
	SpeakerDOUT = machine.GPIO13
)

// Internal I2C bus.
const (
	sysSDA = machine.GPIO12
	sysSCL = machine.GPIO11

	aw9523Addr  = 0x58
	aw88298Addr = 0x36
	axp2101Addr = 0x34

	aw9523OutputP0  = 0x02
	aw9523ConfigP0  = 0x04   // 1 = input, 0 = output
	aw9523GCR       = 0x11   // bit 4: P0 push-pull
	aw9523LEDModeP0 = 0x12   // 1 = GPIO mode, 0 = LED mode
	aw88298ResetBit = 1 << 2 // P0_2
)

var sysI2C = machine.I2C0

// NewSpeaker enables the CoreS3's speaker amp for sampleRate (which must be
// the audio rate you then pass to engine.Start) and returns the I2S backend
// that drives it.
func NewSpeaker(sampleRate uint32) (*esp32s3i2s.Output, error) {
	if err := EnableSpeaker(sampleRate); err != nil {
		return nil, err
	}
	return esp32s3i2s.New(esp32s3i2s.Config{BCLK: SpeakerBCLK, WS: SpeakerWS, DOUT: SpeakerDOUT})
}

// EnableSpeaker releases the AW88298 from reset and configures it for
// 16 bit, 32fs-BCLK I2S at sampleRate. NewSpeaker calls it for you; use it
// directly only if you're creating the esp32s3i2s.Output yourself (e.g.
// with a non-default Config).
func EnableSpeaker(sampleRate uint32) error {
	if err := sysI2C.Configure(machine.I2CConfig{SDA: sysSDA, SCL: sysSCL, Frequency: 400_000}); err != nil {
		return err
	}
	if ctrl, err := readReg8(axp2101Addr, 0x90); err != nil || ctrl&1 == 0 {
		println("cores3: warning: AXP2101 ALDO1 (the AW88298's 1.8V supply) looks off; the speaker may stay silent")
	}

	// AW9523B P0_2 (AW88298 reset): drive it high as a push-pull GPIO output.
	for _, step := range []struct{ reg, set, clear uint8 }{
		{aw9523OutputP0, aw88298ResetBit, 0},
		{aw9523LEDModeP0, aw88298ResetBit, 0},
		{aw9523GCR, 1 << 4, 0},
		{aw9523ConfigP0, 0, aw88298ResetBit},
	} {
		if err := updateReg8(aw9523Addr, step.reg, step.set, step.clear); err != nil {
			return err
		}
	}

	for _, w := range []struct {
		reg   uint8
		value uint16
	}{
		{0x61, 0x0673}, // boost mode disabled
		{0x04, 0x4040}, // I2SEN=1 AMPPD=0 PWDN=0
		{0x05, 0x0008}, // RMSE=0 HAGCE=0 HDCCE=0 HMUTE=0
		{0x06, 0x14C0 | ampInputLevel | aw88298RateCode(sampleRate)}, // I2S on, left channel, Philips, BCK 16*2, -6dB input, sample rate
		{0x0C, ampVolume}, // volume -12dB, see ampVolume
	} {
		if err := writeAW88298(w.reg, w.value); err != nil {
			return err
		}
	}
	return nil
}

// aw88298RateCode is AW88298 register 0x06's sample rate field, picked the
// way M5Unified does: the nearest supported rate at or above sampleRate
// (8k, 11.025k, 12k, 16k, 22.05k, 24k, 32k, 44.1k, 48k, 96k).
func aw88298RateCode(sampleRate uint32) uint16 {
	table := [...]uint32{4, 5, 6, 8, 10, 11, 15, 20, 22, 44}
	rate := (sampleRate + 1102) / 2205
	code := uint16(0)
	for code < uint16(len(table)-1) && rate > table[code] {
		code++
	}
	return code
}

func readReg8(addr uint16, reg uint8) (uint8, error) {
	var r [1]byte
	err := sysI2C.Tx(addr, []byte{reg}, r[:])
	return r[0], err
}

func writeReg8(addr uint16, reg, value uint8) error {
	return sysI2C.Tx(addr, []byte{reg, value}, nil)
}

// updateReg8 sets the bits in set and clears those in clear, leaving the rest alone.
func updateReg8(addr uint16, reg, set, clear uint8) error {
	v, err := readReg8(addr, reg)
	if err != nil {
		return err
	}
	return writeReg8(addr, reg, v&^clear|set)
}

func writeAW88298(reg uint8, value uint16) error {
	return sysI2C.Tx(aw88298Addr, []byte{reg, byte(value >> 8), byte(value)}, nil)
}

// SpeakerStatus is a snapshot of the speaker amp's state, for working out
// why a sketch is silent or crackles. Bits per the AW88298 datasheet
// (Awinic, V1.6), register 0x01 SYSST.
type SpeakerStatus struct {
	SupplyOn  bool   // AXP2101 ALDO1, the amp's 1.8V supply, is on
	Enabled   bool   // the amp is powered up with its I2S input on (SYSCTRL)
	PLLLocked bool   // the amp's PLL is locked to the incoming I2S clock (SYSST bit 0)
	ClockOK   bool   // the amp's clock is stable (SYSST bit 4)
	NoClock   bool   // no I2S clock is arriving (SYSST bit 5)
	Switching bool   // the output stage is switching, i.e. playing (SYSST bit 8)
	Clipping  bool   // the amp is clipping: the signal is too loud for it (SYSST bit 7)
	SYSST     uint16 // the raw status register
	Err       error  // set if the chips couldn't be read
}

// ReadSpeakerStatus reads the amp's (and its supply's) state over the
// internal I2C bus. Call it after NewSpeaker or EnableSpeaker.
func ReadSpeakerStatus() SpeakerStatus {
	var st SpeakerStatus
	ldo, err := readReg8(axp2101Addr, 0x90)
	if err != nil {
		st.Err = err
		return st
	}
	st.SupplyOn = ldo&1 != 0
	sysctrl, err := readAW88298(0x04)
	if err != nil {
		st.Err = err
		return st
	}
	st.Enabled = sysctrl&0x43 == 0x40 // I2SEN=1, AMPPD=0, PWDN=0
	if st.SYSST, err = readAW88298(0x01); err != nil {
		st.Err = err
		return st
	}
	st.PLLLocked = st.SYSST&(1<<0) != 0
	st.ClockOK = st.SYSST&(1<<4) != 0
	st.NoClock = st.SYSST&(1<<5) != 0
	st.Switching = st.SYSST&(1<<8) != 0
	st.Clipping = st.SYSST&(1<<7) != 0
	return st
}

func readAW88298(reg uint8) (uint16, error) {
	var r [2]byte
	err := sysI2C.Tx(aw88298Addr, []byte{reg}, r[:])
	return uint16(r[0])<<8 | uint16(r[1]), err
}
