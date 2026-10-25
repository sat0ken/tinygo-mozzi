// Package esp32s3i2s is an mozzi/engine.Backend for the ESP32-S3 driving an
// I2S DAC/amplifier -- e.g. a MAX98357A, or the AW88298 speaker amp built
// into an M5Stack CoreS3 (see mozzi/board/cores3 for that board's extra
// amp setup).
//
// Unlike the RP2040 (whose PIO block can be fed one sample at a time), the
// ESP32-S3's I2S peripheral has no CPU-writable sample register: it only
// takes data through GDMA. So this backend keeps a ring of DMA buffers
// (linked descriptors pointing back to the first) playing continuously, and
// uses the GDMA "out EOF" interrupt -- raised each time the hardware has
// finished sending one buffer -- to refill that buffer with fresh samples
// from pull(), well before the ring comes back round to it. Like
// backend/esp32s3pwm (and unlike backend/rp2040i2s) this is interrupt
// driven, so it doesn't depend on goroutine scheduling at all.
//
// The I2S0 and GDMA register setup is adapted from the i2s package of the
// TinyGo Conf 2026 badge firmware
// (https://github.com/tinygo-keeb/tinygo-conf-2026-badge, firmware/i2s),
// used with its author's permission; that code in turn follows ESP-IDF
// v5.1's hal/esp32s3/i2s_ll.h, gdma_ll.h and driver/i2s/i2s_std.c. The
// format is Philips-standard I2S, 16 bit samples, 2 channels, with the same
// (mono) sample on both.
package esp32s3i2s

import (
	"device/esp"
	"errors"
	"machine"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"

	"mozzi/audioout"
)

// GPIO matrix output signal numbers (soc/esp32s3/gpio_sig_map.h).
const (
	sigI2S0OBCKOut = 22 // I2S0O_BCK_OUT_IDX
	sigI2S0OWSOut  = 24 // I2S0O_WS_OUT_IDX
	sigI2S0OSDOut  = 25 // I2S0O_SD_OUT_IDX
)

const (
	gdmaPeriphI2S0 = 3 // SOC_GDMA_TRIG_PERIPH_I2S0
	pllClock       = 160_000_000
	mclkDividerMax = 511 // I2S_LL_MCLK_DIVIDER_MAX (9 bit)

	// dmaCPUInterrupt is the Xtensa CPU interrupt line the GDMA channel 0
	// "out" interrupt is mapped to. TinyGo 0.42 uses lines 8-10 itself (USB,
	// its runtime timer, GPIO) and backend/esp32s3pwm uses 12; 13 is the next
	// free level-triggered, priority-1 peripheral line.
	dmaCPUInterrupt = 13
)

// ErrInvalidConfig is returned by New for an unusable Config.
var ErrInvalidConfig = errors.New("esp32s3i2s: invalid config")

// Config describes the I2S wiring and DMA buffering.
type Config struct {
	BCLK machine.Pin // bit clock
	WS   machine.Pin // word select (LRCLK / LRC)
	DOUT machine.Pin // serial data out (DIN on the amp/DAC)

	// SlotBits is the number of bit clocks per channel: 16 (a 32fs bit
	// clock) or 32 (64fs). Samples are always 16 bit; with 32 the extra low
	// bits are zero. 0 means 16.
	SlotBits int
	// BufferFrames is the number of stereo frames per DMA buffer, i.e. how
	// many samples each refill interrupt pulls. Must not exceed 256 (the
	// engine's own output buffer); 0 means 64.
	BufferFrames int
	// BufferCount is the number of DMA buffers in the ring, at least 2. The
	// ring holds BufferCount*BufferFrames frames of output latency. 0 means 4.
	BufferCount int
}

// dmaDescriptor is a GDMA linked-list descriptor (hal/dma_types.h's
// dma_descriptor_t): 12 bytes, 4-byte aligned.
type dmaDescriptor struct {
	dw0    volatile.Register32 // size[11:0] length[23:12] err_eof[28] suc_eof[30] owner[31]
	buffer uintptr
	next   uintptr
}

const (
	dmaSucEOF = 1 << 30
	dmaOwner  = 1 << 31
)

// Output drives an I2S DAC/amplifier from I2S0, always writing the same
// (mono) sample to both channels.
type Output struct {
	cfg   Config
	descs []dmaDescriptor
	bufs  [][]uint32 // one uint32 per frame: left sample in the low half, right in the high half
	next  int        // the next buffer to refill, i.e. the oldest one still playing or queued
	pull  func() audioout.Storage
	gain  int32 // multiplier from a Bits-bit sample to a 16 bit PCM word, see SetVolume
}

// upshift scales the logical Bits-bit sample up to a full 16 bit PCM word.
const upshift = 16 - audioout.Bits

// SetVolume sets the output level, from 0 (silent) to 255 (full scale, the
// default). It scales amplitude linearly -- 128 is about half, i.e. -6dB --
// and it's applied while widening the 8 bit sample into the 16 bit I2S
// word, so turning it down costs none of the sample's resolution (unlike
// scaling the sample in your sketch's UpdateAudio). Safe to call at any
// time, including while playing.
func (o *Output) SetVolume(volume uint8) {
	v := int32(volume)
	o.gain = (v + v>>7) << upshift >> 8 // 0..255 -> 0..256, 256 being 1<<upshift
}

// New validates cfg and allocates the DMA ring (filled with silence). It
// doesn't touch the hardware; Start (called by engine.Start) does.
func New(cfg Config) (*Output, error) {
	if cfg.SlotBits == 0 {
		cfg.SlotBits = 16
	}
	if cfg.BufferFrames == 0 {
		cfg.BufferFrames = 64
	}
	if cfg.BufferCount == 0 {
		cfg.BufferCount = 4
	}
	if cfg.BCLK == machine.NoPin || cfg.WS == machine.NoPin || cfg.DOUT == machine.NoPin ||
		(cfg.SlotBits != 16 && cfg.SlotBits != 32) ||
		cfg.BufferFrames < 1 || cfg.BufferFrames > 256 || cfg.BufferCount < 2 {
		return nil, ErrInvalidConfig
	}

	o := &Output{cfg: cfg}
	o.SetVolume(255)
	n := cfg.BufferCount
	bufBytes := uint32(cfg.BufferFrames * 4)
	o.descs = make([]dmaDescriptor, n)
	o.bufs = make([][]uint32, n)
	for i := range o.descs {
		o.bufs[i] = make([]uint32, cfg.BufferFrames)
		o.descs[i].buffer = uintptr(unsafe.Pointer(&o.bufs[i][0]))
		o.descs[i].next = uintptr(unsafe.Pointer(&o.descs[(i+1)%n]))
		o.descs[i].dw0.Set(bufBytes | bufBytes<<12 | dmaSucEOF | dmaOwner)
	}
	return o, nil
}

// activeOutput is the Output the GDMA interrupt refills. interrupt.New needs
// a plain function, not a method value, hence this singleton (there's only
// one I2S0/GDMA channel 0 to drive anyway).
var activeOutput *Output

// Start implements engine.Backend: it configures I2S0 for sampleRateHz,
// starts GDMA channel 0 looping over the buffer ring, and refills each
// buffer with pull()ed samples from the GDMA interrupt as soon as it has
// been sent.
func (o *Output) Start(sampleRateHz uint32, pull func() audioout.Storage) {
	if sampleRateHz == 0 || pllClock/(sampleRateHz*256) < 2 {
		panic("esp32s3i2s: unsupported sample rate")
	}
	o.pull = pull
	activeOutput = o
	// Restart from a silent ring (Start may be resuming after Stop).
	o.next = 0
	for _, buf := range o.bufs {
		clear(buf)
	}

	o.initI2S(sampleRateHz)
	initGDMA()
	routeOutput(o.cfg.BCLK, sigI2S0OBCKOut)
	routeOutput(o.cfg.WS, sigI2S0OWSOut)
	routeOutput(o.cfg.DOUT, sigI2S0OSDOut)

	esp.INTERRUPT_CORE0.SetDMA_OUT_CH0_INT_MAP(dmaCPUInterrupt)
	interrupt.New(dmaCPUInterrupt, handleDMAInterrupt).Enable()

	o.start()
}

// Stop implements engine.Stopper: it stops I2S0 and the GDMA channel, and
// with them the refill interrupt. Start resumes.
func (o *Output) Stop() {
	esp.DMA.SetOUT_INT_ENA_CH0_OUT_EOF(0)
	esp.I2S0.SetTX_CONF_TX_START(0)
	esp.DMA.SetOUT_LINK_CH0_OUTLINK_STOP(1)
}

func handleDMAInterrupt(interrupt.Interrupt) { activeOutput.isr() }

func (o *Output) isr() {
	esp.DMA.SetOUT_INT_CLR_CH0_OUT_EOF(1)

	// OUT_EOF_DES_ADDR is the descriptor whose buffer was sent most
	// recently. Refill every buffer from o.next up to and including it --
	// normally exactly one, but this also catches up if this interrupt was
	// ever held off for longer than a buffer, and does nothing on a
	// spurious interrupt (when o.next is already past it).
	done := o.descIndex(esp.DMA.GetOUT_EOF_DES_ADDR_CH0())
	if done < 0 {
		return
	}
	n := len(o.bufs)
	for stop := (done + 1) % n; o.next != stop; o.next = (o.next + 1) % n {
		o.fill(o.bufs[o.next])
	}
}

func (o *Output) descIndex(addr uint32) int {
	base := uint32(uintptr(unsafe.Pointer(&o.descs[0])))
	size := uint32(unsafe.Sizeof(dmaDescriptor{}))
	if addr < base || (addr-base)%size != 0 || (addr-base)/size >= uint32(len(o.descs)) {
		return -1
	}
	return int((addr - base) / size)
}

func (o *Output) fill(buf []uint32) {
	gain := o.gain
	for i := range buf {
		sample := int32(o.pull())
		if sample < -audioout.Bias {
			sample = -audioout.Bias
		} else if sample > audioout.Bias-1 {
			sample = audioout.Bias - 1
		}
		pcm := uint32(uint16(int16(sample * gain))) // |sample*gain| <= 128<<upshift: fits int16
		buf[i] = pcm | pcm<<16                      // same sample on both channels
	}
}

// initI2S sets I2S0 up as a Philips-standard, 16 bit, 2 slot TX master
// (i2s_hal_std_set_tx_slot / i2s_hal_set_tx_clock).
func (o *Output) initI2S(sampleRateHz uint32) {
	hw := esp.I2S0

	esp.SYSTEM.SetPERIP_CLK_EN0_I2S0_CLK_EN(1)
	esp.SYSTEM.SetPERIP_RST_EN0_I2S0_RST(1)
	esp.SYSTEM.SetPERIP_RST_EN0_I2S0_RST(0)
	hw.SetTX_CLKM_CONF_CLK_EN(1)

	slot := uint32(o.cfg.SlotBits)
	hw.SetTX_CONF_TX_RESET(1)
	hw.SetTX_CONF_TX_RESET(0)
	hw.SetTX_CONF_TX_SLAVE_MOD(0)
	hw.SetTX_CONF1_TX_BITS_MOD(16 - 1)        // 16 bit data
	hw.SetTX_CONF1_TX_TDM_CHAN_BITS(slot - 1) // slot width
	hw.SetTX_CONF1_TX_MSB_SHIFT(1)            // Philips: data starts one BCLK after WS changes
	hw.SetTX_CONF1_TX_TDM_WS_WIDTH(slot - 1)  // WS half period = one slot
	hw.SetTX_CONF_TX_MONO(0)
	hw.SetTX_CONF_TX_CHAN_EQUAL(0)
	// 2 slots in total, only ch0/ch1 enabled (all 16 are enabled at reset)
	hw.TX_TDM_CTRL.Set(hw.TX_TDM_CTRL.Get()&0xffff0000 | 0x3)
	hw.SetTX_TDM_CTRL_TX_TDM_TOT_CHAN_NUM(2 - 1)
	hw.SetTX_TDM_CTRL_TX_TDM_SKIP_MSK_EN(0)
	hw.SetTX_CONF1_TX_HALF_SAMPLE_BITS(slot - 1)
	hw.SetTX_CONF_TX_WS_IDLE_POL(0)
	hw.SetTX_CONF_TX_BIT_ORDER(0)
	hw.SetTX_CONF_TX_LEFT_ALIGN(0)
	hw.SetTX_CONF_TX_BIG_ENDIAN(0)
	// standard (TDM) mode, no PCM compression
	hw.SetTX_CONF_TX_PDM_EN(0)
	hw.SetTX_CONF_TX_TDM_EN(1)
	hw.SetTX_PCM2PDM_CONF_PCM2PDM_CONV_EN(0)
	hw.SetTX_CONF_TX_PCM_CONF(0)
	hw.SetTX_CONF_TX_PCM_BYPASS(1)

	// Clocks: divide the 160MHz PLL down to MCLK = fs*256, then BCLK = fs*2*SlotBits.
	hw.SetTX_CLKM_CONF_TX_CLK_ACTIVE(1)
	hw.SetRX_CLKM_CONF_MCLK_SEL(0)
	hw.SetTX_CLKM_CONF_TX_CLK_SEL(2) // PLL_F160M
	mclk := sampleRateHz * 256
	bclk := sampleRateHz * 2 * slot
	integ, numer, denom := calcMCLKDiv(pllClock, mclk)
	// i2s_ll_tx_set_mclk: write an impossible divider first, to avoid an
	// error when switching down from a higher rate, then the real one.
	setTXClkDiv(7, 317, 7, 3, 0)
	var x, y, z, yn1 uint32
	if numer != 0 && denom != 0 {
		if numer*2 > denom {
			yn1 = 1
			z = denom - numer
		} else {
			z = numer
		}
		x = denom/z - 1
		y = denom % z
	}
	setTXClkDiv(integ, x, y, z, yn1)
	hw.SetTX_CONF1_TX_BCK_DIV_NUM(mclk/bclk - 1)
}

func setTXClkDiv(integ, x, y, z, yn1 uint32) {
	esp.I2S0.SetTX_CLKM_CONF_TX_CLKM_DIV_NUM(integ)
	esp.I2S0.TX_CLKM_DIV_CONF.Set(z | y<<9 | x<<18 | yn1<<27)
}

// calcMCLKDiv approximates sclk/mclk as integ + numer/denom
// (i2s_hal_calc_mclk_precise_division, in integer arithmetic).
func calcMCLKDiv(sclk, mclk uint32) (integ, numer, denom uint32) {
	integ = sclk / mclk
	diff := sclk % mclk
	denom = 1
	if diff == 0 {
		return
	}
	// round up if the fraction exceeds 1 - 1/(2*max)
	if uint64(diff)*uint64(2*mclkDividerMax) > uint64(mclk)*uint64(2*mclkDividerMax-1) {
		integ++
		return
	}
	best := uint64(1) << 62
	for a := uint32(2); a <= mclkDividerMax; a++ {
		b := uint32((uint64(a)*uint64(diff) + uint64(mclk)/2) / uint64(mclk))
		ma := uint64(diff) * uint64(a)
		mb := uint64(mclk) * uint64(b)
		var e uint64
		if ma > mb {
			e = ma - mb
		} else {
			e = mb - ma
		}
		if e < best {
			best = e
			denom, numer = a, b
			if e == 0 {
				break
			}
		}
	}
	return
}

// initGDMA connects GDMA channel 0's TX side to I2S0, with its EOF
// interrupt (one per sent buffer) enabled.
func initGDMA() {
	dma := esp.DMA
	esp.SYSTEM.SetPERIP_CLK_EN1_DMA_CLK_EN(1)
	esp.SYSTEM.SetPERIP_RST_EN1_DMA_RST(1)
	esp.SYSTEM.SetPERIP_RST_EN1_DMA_RST(0)
	dma.SetMISC_CONF_CLK_EN(1)

	dma.SetOUT_CONF0_CH0_OUT_RST(1)
	dma.SetOUT_CONF0_CH0_OUT_RST(0)
	dma.SetOUT_CONF0_CH0_OUTDSCR_BURST_EN(1)
	dma.SetOUT_CONF0_CH0_OUT_DATA_BURST_EN(1)
	dma.SetOUT_CONF0_CH0_OUT_EOF_MODE(1) // EOF once a buffer's data has left the DMA FIFO
	dma.SetOUT_CONF0_CH0_OUT_AUTO_WRBACK(0)
	dma.SetOUT_CONF1_CH0_OUT_CHECK_OWNER(0)
	dma.SetOUT_PRI_CH0_TX_PRI(0)
	dma.SetOUT_PERI_SEL_CH0_PERI_OUT_SEL(gdmaPeriphI2S0)
	dma.OUT_INT_ENA_CH0.Set(0)
	dma.OUT_INT_CLR_CH0.Set(0xffffffff)
	dma.SetOUT_INT_ENA_CH0_OUT_EOF(1)
}

// routeOutput connects pin to an output signal through the GPIO matrix.
func routeOutput(pin machine.Pin, signal uint32) {
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	reg := (*volatile.Register32)(unsafe.Add(unsafe.Pointer(&esp.GPIO.FUNC0_OUT_SEL_CFG), uintptr(pin)*4))
	reg.Set(signal)
}

// start resets I2S0 TX and GDMA channel 0 and starts them on the ring
// (i2s_tx_channel_start).
func (o *Output) start() {
	hw := esp.I2S0
	dma := esp.DMA
	hw.SetTX_CONF_TX_RESET(1)
	hw.SetTX_CONF_TX_RESET(0)
	dma.SetOUT_CONF0_CH0_OUT_RST(1)
	dma.SetOUT_CONF0_CH0_OUT_RST(0)
	hw.SetTX_CONF_TX_FIFO_RESET(1)
	hw.SetTX_CONF_TX_FIFO_RESET(0)
	dma.OUT_INT_CLR_CH0.Set(0xffffffff)
	dma.SetOUT_LINK_CH0_OUTLINK_ADDR(uint32(uintptr(unsafe.Pointer(&o.descs[0]))) & 0xfffff)
	dma.SetOUT_LINK_CH0_OUTLINK_START(1)
	hw.SetTX_CONF_TX_UPDATE(1)
	for i := 0; i < 100000 && hw.GetTX_CONF_TX_UPDATE() != 0; i++ {
	}
	hw.SetTX_CONF_TX_START(1)
}
