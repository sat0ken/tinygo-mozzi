// Package rp2040pwm is an mozzi/engine.Backend for the Raspberry Pi Pico
// (RP2040): it plays audio out of a single GPIO pin by using the pin's PWM
// duty cycle as a fast one-bit-DAC, the same basic trick Mozzi's classic
// AVR MOZZI_OUTPUT_PWM backend uses. You'll want a simple RC low-pass
// filter (e.g. 1k resistor + 10nF capacitor) between the pin and your
// amplifier/speaker to smooth the PWM carrier back into an analogue signal.
//
// Unlike upstream's RP2040 backend (which relies on the arduino-pico core's
// PWMAudio + DMA), this is a from-scratch implementation on top of TinyGo's
// machine package plus a hand-rolled repeating alarm interrupt (TinyGo does
// not currently expose a repeating hardware timer API), since TinyGo
// sketches don't have the Arduino core underneath them. Architecturally, it
// plays the same role as MozziGuts_impl_RP2040.hpp's MOZZI_OUTPUT_PWM mode.
package rp2040pwm

import (
	"device/rp"
	"machine"
	"runtime/interrupt"

	"mozzi/audioout"
)

// pwmChannel is the subset of *machine.PWM0 (etc)'s API this package needs;
// declared locally because the concrete pwmGroup type in package machine is
// unexported.
type pwmChannel interface {
	Configure(config machine.PWMConfig) error
	Channel(pin machine.Pin) (uint8, error)
	Set(channel uint8, value uint32)
	Top() uint32
}

// fracBits is the sub-tick fixed-point precision used to accumulate the
// sample clock, so that rounding in ticks-per-sample doesn't drift the
// audio rate over time (same concern upstream's
// next_audio_update_shifted/micros_per_update_shifted addresses for
// MOZZI_OUTPUT_EXTERNAL_TIMED).
const fracBits = 8

// Output is an engine.Backend that outputs audio as PWM duty cycle on one pin.
type Output struct {
	pwm     pwmChannel
	channel uint8
	top     uint32

	pull func() audioout.Storage

	periodTicks uint64 // sample period, in timer ticks (microseconds) scaled by 1<<fracBits
	nextTicks   uint64 // accumulator, same scale
}

// New configures pin for PWM output at approximately carrierHz (a
// comfortably-inaudible PWM carrier frequency, well above your intended
// audio rate -- 250_000 is a reasonable default). The resulting resolution
// (Output.Bits) depends on the CPU frequency and carrierHz: roughly
// log2(CPUFrequency/carrierHz) bits. A slower carrier gives more
// resolution, at the cost of being easier to hear/needing a heavier filter.
func New(pin machine.Pin, carrierHz uint32) (*Output, error) {
	slice, err := machine.PWMPeripheral(pin)
	if err != nil {
		return nil, err
	}
	pwm := pwmGroupFor(slice)
	if err := pwm.Configure(machine.PWMConfig{Period: uint64(1e9) / uint64(carrierHz)}); err != nil {
		return nil, err
	}
	channel, err := pwm.Channel(pin)
	if err != nil {
		return nil, err
	}
	// Note: upstream's RP2040 backend also raises the pin's drive strength
	// to 12mA here; TinyGo's machine package does not currently expose
	// that control, so this is left at its (perfectly usable) default.
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
// drive the single TIMER ALARM1 interrupt at a time anyway).
var activeOutput *Output

// Start implements engine.Backend: it arms a repeating RP2040 hardware
// timer interrupt (TIMER ALARM1) at sampleRateHz, calling pull() from that
// interrupt to get each sample and writing it out as PWM duty cycle.
func (o *Output) Start(sampleRateHz uint32, pull func() audioout.Storage) {
	o.pull = pull
	o.periodTicks = (uint64(1000000) << fracBits) / uint64(sampleRateHz)

	now := rp.TIMER.TIMERAWL.Get()
	o.nextTicks = uint64(now) << fracBits

	activeOutput = o
	interrupt.New(rp.IRQ_TIMER_IRQ_1, handleTimerInterrupt).Enable()
	rp.TIMER.SetINTE_ALARM_1(1)
	o.armNext()
}

// Stop implements engine.Stopper: it stops the sample interrupt (the PWM
// pin keeps its last duty cycle). Start resumes.
func (o *Output) Stop() {
	rp.TIMER.SetINTE_ALARM_1(0)
	rp.TIMER.SetINTR_ALARM_1(1) // drop an alarm that may already be pending
}

func handleTimerInterrupt(interrupt.Interrupt) { activeOutput.isr() }

func (o *Output) armNext() {
	o.nextTicks += o.periodTicks
	rp.TIMER.ALARM1.Set(uint32(o.nextTicks >> fracBits))
}

func (o *Output) isr() {
	for {
		rp.TIMER.SetINTR_ALARM_1(1) // write-1-to-clear the pending alarm

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

		o.armNext()
		target := uint32(o.nextTicks >> fracBits)
		// If the just-armed target has already passed (this interrupt, or
		// the code above, ran long), keep pushing samples to catch up
		// rather than let the audio rate drift -- mirrors upstream's
		// audioOutputCallback do/while loop for MOZZI_OUTPUT_EXTERNAL_TIMED.
		if int32(target-rp.TIMER.TIMERAWL.Get()) > 0 {
			return
		}
	}
}

func pwmGroupFor(slice uint8) pwmChannel {
	switch slice {
	case 0:
		return machine.PWM0
	case 1:
		return machine.PWM1
	case 2:
		return machine.PWM2
	case 3:
		return machine.PWM3
	case 4:
		return machine.PWM4
	case 5:
		return machine.PWM5
	case 6:
		return machine.PWM6
	default:
		return machine.PWM7
	}
}
