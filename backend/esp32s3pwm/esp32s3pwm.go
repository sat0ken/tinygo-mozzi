// Package esp32s3pwm is an mozzi/engine.Backend for the ESP32-S3: it plays
// audio out of a single GPIO pin using the LEDC peripheral's PWM duty cycle
// as a fast one-bit-DAC, the same trick backend/rp2040pwm uses on the
// RP2040 (and classic Mozzi's MOZZI_OUTPUT_PWM mode uses on AVR). You'll
// want a simple RC low-pass filter (e.g. 1k resistor + 10nF capacitor)
// between the pin and your amplifier/speaker to smooth the PWM carrier back
// into an analogue signal.
//
// This is ESP32-S3 only (Xtensa LX7): the register layouts and interrupt
// matrix this file pokes directly are chip-specific, and TinyGo 0.41 does
// not yet expose a PWM driver or a supported interrupt.New for the classic
// (non-S3/C3) ESP32. See ../../README.md for the chip support matrix this
// was checked against.
//
// The sample clock is TIMG0's Timer1 (T1), configured as a free-running,
// auto-reloading hardware timer -- deliberately *not* Timer0 (T0), which
// TinyGo's own runtime uses for time.Now()/time.Sleep (see
// runtime/runtime_esp32sx.go). The resulting periodic interrupt is mapped
// to Xtensa CPU interrupt line 12 via the interrupt matrix; lines 8-10 are
// already claimed elsewhere in TinyGo 0.41 (USB, that same runtime timer,
// and GPIO), so 12 was picked to stay clear of all of them.
package esp32s3pwm

import (
	"device/esp"
	"machine"
	"runtime/interrupt"

	"mozzi/audioout"
)

// pwmChannel is the subset of *machine.LEDCPWM's API this package needs.
type pwmChannel interface {
	Configure(config machine.PWMConfig) error
	Channel(pin machine.Pin) (uint8, error)
	Set(channel uint8, value uint32)
	Top() uint32
}

const (
	apbClockHz   = 80_000_000
	timerDivider = 80 // -> 1MHz timer tick, same convenient unit backend/rp2040pwm uses
	timerFreqHz  = apbClockHz / timerDivider

	// audioTimerCPUInterrupt is the Xtensa CPU interrupt line this backend
	// maps TIMG0 Timer1's alarm to. See the package doc for why this
	// particular number.
	audioTimerCPUInterrupt = 12
)

// Output is an engine.Backend that outputs audio as LEDC PWM duty cycle on one pin.
type Output struct {
	pwm     pwmChannel
	channel uint8
	top     uint32

	pull func() audioout.Storage
}

// New configures pin for PWM output at approximately carrierHz (a
// comfortably-inaudible PWM carrier frequency, well above your intended
// audio rate -- 250_000 is a reasonable default), using LEDC timer PWM0.
// The resulting resolution (Output.Bits) depends on carrierHz: the LEDC
// driver picks a duty resolution of 8-14 bits depending on frequency (lower
// frequency -> more bits); see machine.LEDCPWM.Configure.
func New(pin machine.Pin, carrierHz uint32) (*Output, error) {
	pwm := machine.PWM0
	if err := pwm.Configure(machine.PWMConfig{Period: uint64(1e9) / uint64(carrierHz)}); err != nil {
		return nil, err
	}
	channel, err := pwm.Channel(pin)
	if err != nil {
		return nil, err
	}
	return &Output{pwm: pwm, channel: channel, top: pwm.Top()}, nil
}

// Bits reports the actual PWM resolution in effect (Top()+1, in bits) --
// informational, e.g. for logging.
func (o *Output) Bits() int {
	bits := 0
	for top := o.top; top > 0; top >>= 1 {
		bits++
	}
	return bits
}

// activeOutput is the Output currently driving the hardware timer
// interrupt. interrupt.New requires a plain, non-closure function, so it
// can't bind directly to a *Output method value; this package-level
// singleton is the standard TinyGo workaround (only one Output can usefully
// drive the single TIMG0/Timer1 interrupt at a time anyway).
var activeOutput *Output

// Start implements engine.Backend: it configures TIMG0 Timer1 as a
// repeating hardware alarm at sampleRateHz, calling pull() from that
// interrupt to get each sample and writing it out as PWM duty cycle.
func (o *Output) Start(sampleRateHz uint32, pull func() audioout.Storage) {
	o.pull = pull
	activeOutput = o

	ticksPerSample := uint32(timerFreqHz / sampleRateHz)
	if ticksPerSample == 0 {
		ticksPerSample = 1
	}

	esp.TIMG0.SetT1CONFIG_EN(0) // stop the timer while we configure it
	esp.TIMG0.SetT1CONFIG_INCREASE(1)
	esp.TIMG0.SetT1CONFIG_DIVIDER(timerDivider)
	esp.TIMG0.SetT1CONFIG_AUTORELOAD(1) // counter resets to the LOAD value on every alarm
	esp.TIMG0.SetT1CONFIG_ALARM_EN(1)   // self-clearing: isr() re-arms it after every alarm

	esp.TIMG0.T1LOADLO.Set(0)
	esp.TIMG0.T1LOADHI.Set(0)
	esp.TIMG0.T1LOAD.Set(0) // any write latches LOADLO/LOADHI into the live counter

	esp.TIMG0.T1ALARMLO.Set(ticksPerSample)
	esp.TIMG0.T1ALARMHI.Set(0)

	esp.TIMG0.SetINT_CLR_TIMERS_T1_INT_CLR(1) // clear any stale pending flag before enabling
	esp.TIMG0.SetINT_ENA_TIMERS_T1_INT_ENA(1)

	esp.INTERRUPT_CORE0.SetTG_T1_INT_MAP(audioTimerCPUInterrupt)
	interrupt.New(audioTimerCPUInterrupt, handleTimerInterrupt).Enable()

	esp.TIMG0.SetT1CONFIG_EN(1)
}

// Stop implements engine.Stopper: it stops the sample timer and its
// interrupt (the PWM pin keeps its last duty cycle). Start resumes.
func (o *Output) Stop() {
	esp.TIMG0.SetT1CONFIG_EN(0)
	esp.TIMG0.SetINT_ENA_TIMERS_T1_INT_ENA(0)
	esp.TIMG0.SetINT_CLR_TIMERS_T1_INT_CLR(1)
}

func handleTimerInterrupt(interrupt.Interrupt) { activeOutput.isr() }

func (o *Output) isr() {
	// Acknowledge the pending flag and re-arm the alarm -- but do *not*
	// disable INT_ENA (unlike runtime_esp32sx.go's one-shot sleepTicks
	// alarm). AUTORELOAD alone is not enough to keep this firing: the
	// hardware clears ALARM_EN itself every time the alarm triggers (it's a
	// self-clearing bit, see the ESP32-S3 TRM's TIMG_TxCONFIG_REG; ESP-IDF's
	// gptimer ISR re-enables it the same way). Without re-setting it here,
	// this interrupt fires exactly once and audio output freezes -- which is
	// what the first real-hardware test of this backend showed.
	esp.TIMG0.SetINT_CLR_TIMERS_T1_INT_CLR(1)
	esp.TIMG0.SetT1CONFIG_ALARM_EN(1)

	sample := int32(o.pull())
	if sample < -audioout.Bias {
		sample = -audioout.Bias
	} else if sample > audioout.Bias-1 {
		sample = audioout.Bias - 1
	}
	duty := uint32(sample+audioout.Bias) * (o.top + 1) / (2 * audioout.Bias)
	if duty > o.top {
		duty = o.top
	}
	o.pwm.Set(o.channel, duty)
}
