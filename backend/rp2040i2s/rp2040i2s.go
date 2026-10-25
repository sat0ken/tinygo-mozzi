// Package rp2040i2s is an mozzi/engine.Backend for RP2040 boards driving an
// external I2S DAC/amplifier -- e.g. a MAX98357A (I2S class-D amp) or a
// PCM5102A (I2S DAC) -- instead of the PWM-as-DAC trick backend/rp2040pwm
// uses. Real hardware like this generally sounds cleaner than PWM-out-a-pin
// and doesn't need an RC filter, at the cost of needing three GPIOs and an
// external chip instead of one GPIO and two passives.
//
// Unlike backend/rp2040pwm (which needs its own hand-rolled timer
// interrupt to get a precise sample clock), the RP2040's PIO block can
// generate a real I2S bitstream in hardware, with the bit clock itself
// providing sample-accurate timing "for free": this backend just needs to
// keep the PIO's output FIFO fed, blocking (yielding to other goroutines)
// whenever it gets ahead. It builds on the community PIO library
// github.com/tinygo-org/pio (specifically piolib.I2S) rather than
// reimplementing I2S-over-PIO from scratch.
//
// Add the dependency with:
//
//	go get github.com/tinygo-org/pio
//
// Note: this backend's output loop runs in a goroutine, not a hardware
// interrupt (unlike backend/rp2040pwm/esp32s3pwm). It only works because
// mozzi/engine.AudioHook() yields to other goroutines on every call -- see
// that package's doc if you're wondering why, or writing a similar backend.
package rp2040i2s

import (
	"machine"

	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"

	"mozzi/audioout"
)

// Output drives an external I2S DAC/amplifier. It always writes a stereo
// stream with the same (mono) sample duplicated on both channels -- exactly
// what a MAX98357A wants: left it up to the amp's SD pin to pick L, R, or
// (the usual wiring, SD floating/pulled to VDD through a resistor) the
// (identical, here) average of the two.
type Output struct {
	i2s *piolib.I2S
	gen uint32 // bumped by Start and Stop; run() exits once it's no longer current
}

// New configures a PIO state machine on pioBlock (pass pio.PIO0 or
// pio.PIO1) for I2S output. dataPin carries the audio bitstream (DIN on a
// MAX98357A); clockPin and clockPin+1 carry the bit clock and word-select
// clock respectively (BCLK and LRC on a MAX98357A) -- the PIO program
// requires them to be two *consecutive* GPIO numbers, clockPin then
// clockPin+1, in that order.
func New(pioBlock *pio.PIO, dataPin, clockPin machine.Pin) (*Output, error) {
	sm, err := pioBlock.ClaimStateMachine()
	if err != nil {
		return nil, err
	}
	i2s, err := piolib.NewI2S(sm, dataPin, clockPin)
	if err != nil {
		return nil, err
	}
	return &Output{i2s: i2s}, nil
}

// Start implements engine.Backend: it sets the I2S bit clock for
// sampleRateHz and starts a goroutine that continuously pulls samples and
// writes them out. The PIO hardware's own FIFO/clock -- not a software
// timer -- paces this to real time: WriteStereo blocks (yielding to other
// goroutines, e.g. your main loop's engine.AudioHook) whenever it gets
// ahead of the hardware.
func (o *Output) Start(sampleRateHz uint32, pull func() audioout.Storage) {
	if err := o.i2s.SetSampleFrequency(sampleRateHz); err != nil {
		panic(err)
	}
	o.gen++
	go o.run(pull, o.gen)
}

// Stop implements engine.Stopper: the output goroutine exits after writing
// the batch it's on. Start resumes.
func (o *Output) Stop() { o.gen++ }

// batchFrames is how many stereo frames run() computes before handing them
// to WriteStereo in one call. WriteStereo/TxPut already blocks (yielding)
// on each frame that arrives before the hardware is ready for it, so this
// doesn't change the real-time pacing -- it only amortizes the per-call
// overhead of pull()+WriteStereo (and the goroutine bookkeeping inside it)
// over more samples, which matters because that overhead is paid on every
// single sample, 16384+ times a second. Picked empirically: small enough
// not to add perceptible latency (8 frames is 0.5ms at 16384Hz), large
// enough to meaningfully cut down how often run() re-enters its loop body.
const batchFrames = 8

func (o *Output) run(pull func() audioout.Storage, gen uint32) {
	frames := make([]uint32, batchFrames)
	const upshift = 16 - audioout.Bits // scales the logical Bits-bit sample up to a full 16 bit PCM word
	for o.gen == gen {
		for i := range frames {
			sample := int32(pull())
			if sample < -audioout.Bias {
				sample = -audioout.Bias
			} else if sample > audioout.Bias-1 {
				sample = audioout.Bias - 1
			}
			pcm := uint32(uint16(int16(sample) << upshift))
			frames[i] = pcm | pcm<<16 // same sample on both channels
		}
		o.i2s.WriteStereo(frames)
	}
}
