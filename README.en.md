# mozzi (TinyGo port)

日本語: [README.md](README.md)

A [TinyGo](https://tinygo.org/) port of [Mozzi](https://github.com/sensorium/Mozzi)
([docs](https://sensorium.github.io/Mozzi/)), the Arduino sound synthesis
library. The original C++ library lives untouched in [`../Mozzi`](../Mozzi)
for reference; this directory (`mozzi/`) is a separate, from-scratch Go
module that ports its architecture and a representative core of its
building blocks, with hardware backends for the **Raspberry Pi Pico
(RP2040)** and the **ESP32-S3**.

## Status: a core port, not a 1:1 transliteration

Upstream Mozzi is a big library: ~60 core headers, ~60 wavetables, ~15
hardware backends (AVR/ESP32/SAMD/RP2040/STM32/Teensy/...), and over 100
example sketches. Porting every file mechanically wasn't a realistic goal
here. Instead, this port:

- Faithfully ports the **core architecture** (the control-rate/audio-rate
  scheduler, fixed-point math, the main oscillator/envelope/filter/delay
  classes) as idiomatic Go.
- Ports **all 59 "flat" wavetables** from `Mozzi/tables/*.h` mechanically
  (see [Wavetables](#wavetables) below) -- every table upstream ships
  outside the `BandLimited_*` directories is available.
- Ships **four hardware backends** (RP2040 PWM output, RP2040 + external
  I2S DAC/amp, ESP32-S3 PWM output, ESP32-S3 + I2S DAC/amp, including the
  M5Stack CoreS3's built-in speaker), picked at build time by a small
  [`board`](board) package so every sketch builds for every board.
- Ships **24 example sketches, laid out like upstream's `examples/`
  folder**: all of upstream's `01.Basics` and `02.Control`, plus FMsynth,
  ADSR_Audio_Rate_Envelope, AudioDelay and ResonantFilter, and four music
  sketches of its own built on `mozzi/song` (not part of upstream Mozzi --
  see [Playing a melody](#playing-a-melody)). See [Examples](#examples).
- Explicitly does **not** port everything else (most sensor input helpers,
  other hardware backends, most sample playback, reverb, MIDI I/O, the newer
  UFix/SFix `FixMath` library, ...) -- see [What's not ported](#whats-not-ported).

The result is meant to be a solid foundation you can build a synth on today,
and a clear, consistent pattern to extend from, not a drop-in replacement
for every Mozzi sketch that exists.

## Requirements

- [TinyGo](https://tinygo.org/) 0.41+ (developed and tested against 0.41.1);
  0.42+ for `backend/esp32s3i2s` (and so for the M5Stack CoreS3), which needs
  the ESP32-S3 GDMA register definitions added in 0.42 (tested with 0.42.0)
- One of the boards below.

## Boards

| Board / chip | Backend | Status |
|---|---|---|
| **Raspberry Pi Pico** and other RP2040 boards, PWM-out-a-pin + RC filter | [`backend/rp2040pwm`](backend/rp2040pwm) | Supported |
| **Any RP2040 board + an external I2S DAC/amp** (e.g. **Waveshare RP2040-Zero + MAX98357A**) | [`backend/rp2040i2s`](backend/rp2040i2s) | Supported |
| **ESP32-S3** (Xtensa LX7), PWM-out-a-pin + RC filter | [`backend/esp32s3pwm`](backend/esp32s3pwm) | Supported |
| **ESP32-S3 + an I2S DAC/amp** (e.g. a MAX98357A, the **M5Stack CoreS3**'s built-in AW88298 speaker amp, or the **TinyGo Conf 2026 badge**) | [`backend/esp32s3i2s`](backend/esp32s3i2s) | Supported (needs TinyGo 0.42+ for the GDMA register definitions). ESP32-S3's I2S peripheral is DMA-only, so this backend loops a ring of GDMA buffers and refills each one from the GDMA "buffer sent" interrupt; see [`docs/04-hardware-backends.md`](docs/04-hardware-backends.md). |
| ESP32-C3 (RISC-V), PWM-out-a-pin | -- | Not shipped, but should be a straightforward port of `esp32s3pwm`: TinyGo 0.41 has the same LEDC PWM driver and a working `interrupt.New` for C3 too. Nobody has needed it here yet. (Its I2S peripheral is the same DMA-only design as the S3's, so an I2S backend for it would follow `esp32s3i2s`.) |
| Classic ESP32 (Xtensa LX6, e.g. plain "ESP32 DevKitC") | -- | **Not currently possible**: TinyGo 0.41's `machine` package has no PWM (or DAC) driver for this chip, and `runtime/interrupt` has no `interrupt.New` support for it either -- both would need writing from scratch against raw Xtensa/`device/esp` registers, unverified by anyone. |
| Everything else Mozzi/TinyGo support (AVR, SAMD, STM32, Teensy, esp8266, ...) | -- | Not ported; `engine.Backend` is the extension point (see [Extending](#extending)). |

The two PWM backends use the same trick: PWM duty cycle as a fast one-bit
DAC (same as classic Mozzi's AVR `MOZZI_OUTPUT_PWM` mode), driven by a
periodic hardware timer interrupt this port sets up by hand (neither RP2040
nor ESP32-S3 has an off-the-shelf "repeating audio timer" API in TinyGo).
`rp2040i2s` instead drives a real I2S DAC/amp via the RP2040's PIO block
(built on the community [`github.com/tinygo-org/pio`](https://github.com/tinygo-org/pio)
library's `piolib.I2S`, rather than reimplementing I2S-over-PIO from
scratch): the I2S bit clock itself paces sample output in hardware, so this
backend runs a goroutine that just keeps the PIO's FIFO fed, instead of a
timer interrupt. `esp32s3i2s` can't do that -- ESP32-S3's I2S only takes
data through GDMA -- so it keeps a ring of DMA buffers playing and refills
each one from the GDMA interrupt that fires when it's been sent.
`engine.Backend` is a one-method interface either way, so
any board with a PWM/DAC/I2S output and *some* way to pace sample delivery
(interrupt or otherwise) is a reasonable target -- see [Extending](#extending).

## Wiring

**PWM backends** (`rp2040pwm`, `esp32s3pwm`): audio comes out of one GPIO
pin as PWM. Put a simple RC low-pass filter between the pin and your
amp/speaker/scope to turn the (~250kHz, well above hearing range) PWM
carrier into an analogue signal:

```
audioPin ---[ 1k ]---+--- to amplifier / speaker / scope
                      |
                    [10nF]
                      |
                     GND
```

Through [`board`](board), the PWM comes out of GPIO15 on RP2040 boards and
GPIO4 on an ESP32-S3 (`-tags esp32s3pwm`); to use another pin, change
`audioPin` in `board/board_rp2040pwm.go` / `board/board_esp32s3pwm.go`.

Driving a bare low-impedance speaker (not a line-in) straight off a GPIO
pin -- even through the RC filter above -- can ask for far more current
than a pin can safely source (an 8 ohm speaker at 3.3V wants ~412mA!). If
that's your setup, add a single NPN transistor as a low-side switch instead, no RC filter needed
(the speaker's own inertia, and the transistor's switching speed, roll off
the PWM carrier on their own):

```
        3V3
         |
      [ speaker ]
         |
       collector
   NPN (2N3904 / 2SC1815 / anything small-signal)
       base ----[ 1k ohm ]---- audioPin
       emitter
         |
        GND
```

(optionally, a diode -- cathode to 3V3, anode to the collector -- across
the speaker absorbs coil kickback; cheap insurance, not required for a quick test)

**`rp2040i2s`** (e.g. a Waveshare RP2040-Zero + a MAX98357A I2S amp): no RC
filter needed, but three GPIOs and a shared ground instead of one:

```
RP2040 board        MAX98357A
------------        ---------
3V3            -->  VIN
GND            -->  GND
GPIO2          -->  DIN   (audio data)
GPIO3 (BCLK)   -->  BCLK
GPIO4 (BCLK+1) -->  LRC
(leave SD unconnected, or pull to VDD via ~1M ohm, for mono L+R-average output)
                     speaker terminals --> your speaker
```

BCLK and LRC must be on two *consecutive* GPIO numbers (BCLK, then
BCLK+1) -- a requirement of the PIO program `rp2040i2s` uses, not of the
MAX98357A itself. Build with `-tags rp2040i2s` (see [Examples](#examples)).

**M5Stack CoreS3**: no wiring -- see [M5Stack CoreS3](#m5stack-cores3).

## Examples

`examples/` mirrors upstream Mozzi's `examples/` folder:
`examples/01.Basics/Sinewave` is the port of
`Mozzi/examples/01.Basics/Sinewave/Sinewave.ino`, and so on. A sketch
doesn't hard-code its hardware; it gets its audio output from
`board.NewOutput`, and [`board`](board) supplies the one for the board you
build for:

| Board | Build flags |
|---|---|
| M5Stack CoreS3 / CoreS3 SE, built-in speaker | `-target=esp32s3-generic -tags cores3` |
| ESP32-S3, PWM out of GPIO4 + RC filter | `-target=esp32s3-generic -tags esp32s3pwm` |
| Raspberry Pi Pico / any RP2040, PWM out of GPIO15 + RC filter | `-target=pico` (or your board's RP2040 target) |
| RP2040 + I2S DAC/amp, e.g. RP2040-Zero + MAX98357A (DIN=GPIO2, BCLK=GPIO3, LRC=GPIO4) | `-target=waveshare-rp2040-zero -tags rp2040i2s` |
| TinyGo Conf 2026 badge (ESP32-S3 DevKit + MAX98357A), its speaker | `-target=esp32s3-generic -tags tinygoconf2026badge` |

For example, from the `mozzi/` directory:

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave   # M5Stack CoreS3
tinygo flash -target=pico ./examples/01.Basics/Sinewave                                                # Raspberry Pi Pico
go mod tidy   # once, before the first -tags rp2040i2s build: fetches github.com/tinygo-org/pio
tinygo flash -target=waveshare-rp2040-zero -tags rp2040i2s ./examples/01.Basics/Sinewave               # RP2040-Zero + MAX98357A
```

Sinewave plays a fixed 440Hz tone -- a good first check that your
toolchain, board and wiring all work. An ESP32-S3 build without one of
its tags (`cores3`, `tinygoconf2026badge` or `esp32s3pwm`) stops with a
compile error naming the fix, rather than guessing: they use different
pins, and the PWM set-up's GPIO4 is the CoreS3's microSD card chip-select
line.

| Example | |
|---|---|
| **01.Basics** | |
| `Sinewave` | a fixed 440Hz sine |
| `Sinewave_HIFI` | the same; upstream's HIFI output mode has no equivalent here (see the file) |
| `Control_Gain` | a sine whose volume steps down at control rate |
| `Skeleton` | the minimal, silent starting point for your own sketch |
| `Skeleton_Multi` | a sketch split over two files |
| `Table_Resolution` | sweeps six sine table sizes in turn, to hear aliasing |
| `Vibrato` | vibrato by phase modulation |
| `Vibrato_Midi_Note` | vibrato by modulating the MIDI note at control rate |
| **02.Control** | |
| `Control_Echo_Theremin` | light-sensor theremin with echoes -- needs a light-dependent resistor on `board.A0` |
| `Control_Oscil_Wash` | an ambient wash of 8 tones with slowly swept volumes |
| `Control_Tremelo` | tremolo, its gain smoothed at audio rate by a Line |
| `EventDelay` | a tone toggled on and off every second |
| `Line_Gliss` | sawtooth glissandos, the phase increment interpolated by a Line |
| `Line_Gliss_Double_32k_HIFI` | two detuned glissandos at a 32768Hz audio rate |
| `Metronome_SampleHuffman` | random Huffman-compressed thumb piano samples on a metronome |
| `Stop_Start` | a button on `board.Button` stops and restarts the engine |
| **06.Synthesis** `FMsynth` | 2-operator FM synth with a wandering pitch |
| **07.Envelopes** `ADSR_Audio_Rate_Envelope` | randomized ADSR-enveloped notes |
| **09.Delays** `AudioDelay` | modulated delay line (flanger-ish) |
| **10.Audio_Filters** `ResonantFilter` | resonant low-pass on a noisy wavetable |
| **TinyGo_Extras** (not from upstream) | |
| `fanfare` | a JRA horse-racing fanfare, transcribed from a recording (`mozzi/song`) |
| `furuhata_op` | a TV theme's opening ostinato (partial -- see the file's doc comment) |
| `fanfare_synth` | the same fanfare on three detuned sawtooths through an ADSR |
| `jukebox` | public-domain tunes (Ode to Joy, Twinkle Twinkle, Für Elise) in two parts, each note shaped by an ADSR; `board.Button` skips to the next tune |

The two sketches that need inputs use pins `board` defines per board:
`board.A0` (where upstream reads analog input 0) and `board.Button` (a
push button, read with `board.ConfigureButton`/`board.ButtonPressed`); see
the [`board`](board) package doc for which GPIOs they are. On the TinyGo
Conf 2026 badge they're already there: the joystick's X axis and SW1.

## TinyGo Conf 2026 badge

The [TinyGo Conf 2026 badge](https://github.com/tinygo-keeb/tinygo-conf-2026-badge)
is an ESP32-S3 DevKit on a board with a MAX98357A I2S amp and a speaker
(BCLK=GPIO45, LRC=GPIO21, DIN=GPIO47), a joystick and two switches. Every
example builds for it with `-tags tinygoconf2026badge`:

```sh
tinygo flash -target=esp32s3-generic -tags tinygoconf2026badge -port /dev/ttyACM0 ./examples/01.Basics/Sinewave
```

- Plug in the DevKit's USB port wired straight to the ESP32-S3 (it shows
  up as Espressif's "USB JTAG/serial debug unit"): that's the one TinyGo
  flashes through and prints to. The other port, through a USB-UART chip
  (e.g. a CH343, 1a86:55d3), shows nothing from `println`.
- The default volume is 32 (of 255), the level the badge's own firmware
  plays at: its MAX98357A runs at 12dB gain and distorts when driven much
  harder. Change it with `-ldflags="-X mozzi/board.volume=64"`, or
  `board.SetVolume` from code.
- `board.A0` is the joystick's X axis (GPIO7; about 512 of 1023 centred,
  its travel limited to roughly a quarter of the range either way), so
  [`Control_Echo_Theremin`](examples/02.Control/Control_Echo_Theremin) is
  played by moving the stick. `board.Button` is SW1 (GPIO13, pulls the pin
  low), so [`Stop_Start`](examples/02.Control/Stop_Start) stops the sound
  for a second when you press it.
- The badge's own firmware suggests `-target=esp32s3-box-3`, for its
  display and other drivers; mozzi needs none of those, and
  `esp32s3-generic` works.

## M5Stack CoreS3

The CoreS3 (and CoreS3 SE) is an ESP32-S3 board with a built-in speaker
behind an AW88298 I2S amp, so it needs no wiring at all: build any example
with `-tags cores3` and it plays through it. [`board/cores3`](board/cores3)
wakes the amp up over the board's internal I2C bus (releasing its reset
line on the AW9523B IO expander and configuring the AW88298, following
M5Unified) and drives it with [`backend/esp32s3i2s`](backend/esp32s3i2s) on
the speaker's I2S pins (BCK=G34 / WS=G33 / DOUT=G13).

**Flashing.** Plug the CoreS3 in over USB-C (it shows up as Espressif's
"USB JTAG/serial debug unit", no driver needed) and, from the `mozzi/`
directory:

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Control_Oscil_Wash
```

- There is no CoreS3-specific TinyGo target; `esp32s3-generic` plus `-tags
  cores3` is the combination to use. Don't build for a CoreS3 with `-tags
  esp32s3pwm`: that drives PWM out of GPIO4, the CoreS3's microSD card
  chip-select line.
- `-port` is the board's serial port: `/dev/ttyACM0` (or `ACM1`, ...) on
  Linux, `/dev/cu.usbmodem*` on macOS, `COMn` on Windows. On Linux your user
  needs to be in the group that owns the port (`dialout`, or `uucp` on
  Arch-based distros); check with `ls -l /dev/ttyACM0`.
- Flashing erases the board's flash first: M5Stack's factory firmware (and
  UIFlow, etc.) is gone after the first flash. M5Burner can put it back.
- The sketch starts right after flashing, and on every power-up.

**Volume.** The output level goes from 0 (silent) to 255 (full scale, the
default) and scales the amplitude linearly -- 128 is about half (-6dB), 64
a quarter (-12dB). Set it for any example at build time, without editing
it:

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -ldflags="-X mozzi/board.volume=128" -port /dev/ttyACM0 ./examples/01.Basics/Sinewave
```

or from code, at any time (e.g. from `UpdateControl`, for a fade), with
`board.SetVolume(128)`. It's applied while the backend widens each 8 bit
sample into the 16 bit I2S word, so turning it down costs no resolution
(scaling the sample in `UpdateAudio` instead would). On the other boards it
has no effect.

Under that, `board/cores3` runs the amp 18dB below M5Unified's setting:
-6dB at its I2S input and -12dB at its volume stage. M5Unified drives the
amp at full gain but never sends it a full-scale signal; mozzi sketches
routinely do, and at full gain that made the amp clip -- its output stage
kept cutting out, heard as popcorn-like crackling instead of a tone. With
the 18dB, a full-scale tone at volume 255 measured clean, so 255 is the
loudest level that doesn't clip.

**Sound quality.** The built-in speaker is small, and a low pure tone --
`Sinewave`'s 440Hz -- sounds a little muddy on it, higher ones clearer.
That's the speaker: M5Stack's own M5Unified `M5.Speaker.tone()` sounds the
same at 440 to 3520Hz, and so does upstream Mozzi; it isn't the 8 bit
samples, the 16384Hz rate or I2S clock jitter either (16 bit samples, 16kHz
and a jitter-free 15625Hz all sounded the same).

**Inputs.** `board.A0` is GPIO8 and `board.Button` is GPIO9, the two
signal pins of the black Grove Port B. Port B's power pin is **5V**, which
the ESP32-S3's pins can't take: wire sensors and buttons to 3.3V from the
M-Bus header instead.

The amp's 1.8V supply (the AXP2101 PMIC's ALDO1) is on by default, so
`board/cores3` only reads it; if it ever prints a warning about it over USB
serial, the speaker will stay silent. Nothing here drives the CoreS3's
display.

**If you hear nothing, or crackling.** First check the flash itself worked: the command
needs `-tags cores3`, and a successful flash ends with `MD5 verified` /
`Device reset.` -- a build error (e.g. one naming
`boardNotSelected_buildWith_tags_cores3_tinygoconf2026badge_or_esp32s3pwm...`) means nothing was
written and the board is still running whatever it ran before. Then flash
[`tools/cores3-check`](tools/cores3-check), which plays a 440Hz tone and
reports every second, over USB serial, on each step of the audio path:

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./tools/cores3-check
tinygo monitor -port /dev/ttyACM0
```

```
---- cores3-check: playing a 440Hz test tone ----
audio out  : 16374 samples/s (expect ~ 16384 ) OK
tone level : 127 of 127 at volume 255 of 255 OK
I2S data   : 141947 edges/s on DOUT (G13) OK
amp supply : AXP2101 ALDO1 on OK
amp        : enabled yes | PLL locked yes | clock stable yes | no clock no OK
amp output : clipping 0 of 11 reads | output stage stopped 0 of 11 OK
```

The last line counts how often, out of ~10 reads a second, the amp
reported clipping or its output stage not running -- crackling shows up
there. If every line says OK, the board is playing, and a silent sketch is down
to what the sketch itself does (some are quiet or silent for stretches by
design -- `EventDelay` is silent every other second, and `Skeleton` always).
Your own sketches can print the same amp report with
`cores3.ReadSpeakerStatus()`.

**Comparing with upstream Mozzi.** Upstream's examples can run on the same
CoreS3, unmodified, for an A/B comparison -- and since `examples/` mirrors
upstream's layout, each one's counterpart is at the same path:
[`tools/cores3-original/build.sh`](tools/cores3-original/build.sh) copies
an example from `../Mozzi/examples`, adds
[`mozzi_cores3.h`](tools/cores3-original/mozzi_cores3.h) to it (I2S on the
speaker pins, the same 16384Hz audio rate as this port -- upstream's ESP32
default is 32768Hz -- and the same amp setup as `board/cores3`), and builds
and flashes it with arduino-cli. It needs arduino-cli with the `esp32:esp32`
core 3.x, and Mozzi 2.x's dependency, the FixMath library:

```sh
# from mozzi/; ARDUINO_CLI defaults to arduino-cli on PATH
ARDUINO_CLI=/path/to/arduino-cli tools/cores3-original/build.sh 01.Basics/Sinewave /dev/ttyACM0    # upstream
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave   # this port
```

(FixMath: `arduino-cli lib install FixMath`, or point `LIBRARIES` at a
checkout of https://github.com/tomcombriat/FixMath.) Leave out the port
argument to build without flashing. Of upstream's `01.Basics` and
`02.Control`, all but the two `_HIFI` sketches build this way (those force
AVR's two-pin PWM output mode, which upstream doesn't support on ESP32).
Two that do build read hardware upstream addresses by Arduino pin number,
which means something else on a CoreS3: `Control_Echo_Theremin` reads
analog pin 0 (GPIO0, not an ADC pin on the ESP32-S3), and `Stop_Start`
reads pin 4 (the microSD chip select) -- and upstream's ESP32 `stopMozzi()`
is unimplemented anyway. The ported versions use `board.A0`/`board.Button`
instead.

Known differences in the output stage: for a mono sketch, upstream's ESP32
I2S output sends the sample on the left channel and 0 on the right, while
`esp32s3i2s` sends it on both. If the amp plays the average of the two
channels, upstream will sound about 6dB quieter; if it plays only one
channel, they'll match. Of the sketches with random elements, those that
keep Mozzi's default random seed (e.g. `ResonantFilter`) produce the same
sequence on both; `ADSR_Audio_Rate_Envelope` reseeds from the hardware RNG
at startup upstream (`randSeed()`), while the port uses a fixed seed, so
their note sequences differ.

## Architecture

Mozzi's core design maps onto this port as follows:

| Mozzi (C++)                              | This port (Go)                          |
|-------------------------------------------|------------------------------------------|
| `setup()` / `loop()` + `audioHook()`      | `main()` + `engine.Start` + `engine.AudioHook()` loop |
| `updateControl()` / `updateAudio()`       | Your type's `UpdateControl()`/`UpdateAudio()` methods (implements `engine.Sketch`) |
| `MozziGuts.hpp` (scheduler, output buffer)| [`engine`](engine) |
| Per-platform `MozziGuts_impl_*.hpp`       | [`backend/rp2040pwm`](backend/rp2040pwm), [`backend/rp2040i2s`](backend/rp2040i2s), [`backend/esp32s3pwm`](backend/esp32s3pwm), [`backend/esp32s3i2s`](backend/esp32s3i2s) (all implement `engine.Backend`) |
| `AudioOutput.h` (`MonoOutput`/`StereoOutput`) | [`audioout`](audioout) |
| `mozzi_fixmath.h`                          | [`fixmath`](fixmath) |
| `mozzi_rand.h`                             | [`rand`](rand) |
| `mozzi_midi.h`                             | [`midi`](midi) |
| `Oscil.h`, `Phasor.h`                      | [`oscil`](oscil) |
| `Line.h`                                   | [`line`](line) (one generic `Line[T]`, see below) |
| `ADSR.h`, `Ead.h`, `Smooth.h`              | [`envelope`](envelope) |
| `EventDelay.h`                             | [`eventdelay`](eventdelay) |
| `Metronome.h`                              | [`metronome`](metronome) |
| `RollingAverage.h`                         | [`rollingaverage`](rollingaverage) |
| `mozzi_analog.h` (`mozziAnalogRead`)       | [`analog`](analog) (a blocking read, as upstream does on ESP32/RP2040) |
| `SampleHuffman.h`, `samples/thumbpiano_huffman/*.h` | [`samplehuffman`](samplehuffman), [`samples/thumbpianohuffman`](samples/thumbpianohuffman) (generated, see [Extending](#extending)) |
| `ResonantFilter.h`                         | [`filter`](filter) |
| `AudioDelay.h`, `AudioDelayFeedback.h`, `ControlDelay.h` | [`delay`](delay) (`ControlDelay` is just an `AudioDelay[T]` used at control rate) |
| `tables/*.h`                               | [`tables`](tables) (generated, see below) |
| *(no upstream equivalent)*                  | [`song`](song) -- a small monophonic melody sequencer, see [Playing a melody](#playing-a-melody) |
| Board selection (`MOZZI_AUDIO_MODE` etc.)    | [`board`](board) -- picks the backend and input pins at build time, see [Examples](#examples); [`board/cores3`](board/cores3) sets up the M5Stack CoreS3's speaker |

A classic Mozzi sketch:

```cpp
Oscil <SIN2048_NUM_CELLS, MOZZI_AUDIO_RATE> aSin(SIN2048_DATA);

void setup(){ startMozzi(); aSin.setFreq(440); }
void updateControl(){}
AudioOutput updateAudio(){ return MonoOutput::from8Bit(aSin.next()); }
void loop(){ audioHook(); }
```

becomes:

```go
type sketch struct{ aSin *oscil.Oscil }
func (s *sketch) UpdateControl() {}
func (s *sketch) UpdateAudio() audioout.Mono { return audioout.From8Bit(int16(s.aSin.Next())) }

func main() {
    s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
    s.aSin.SetFreq(440)
    out, _ := board.NewOutput(audioRate)
    engine.Start(s, audioRate, controlRate, out)
    for { engine.AudioHook() }
}
```

See [`examples/01.Basics/Sinewave`](examples/01.Basics/Sinewave) for the runnable version.

### Why templates became fields (and generics)

Upstream leans heavily on C++ templates (`Oscil<NUM_CELLS, RATE>`,
`Line<T>`, `ADSR<RATE1, RATE2, T>`, ...) to constant-fold shift/divide math
at compile time -- a real win on an 8-bit AVR at a few MHz. On an RP2040
(dual Cortex-M0+ at 133MHz+), that headroom is no longer the constraint, so
most of those template parameters became ordinary constructor
arguments/fields (see e.g. [`oscil.New`](oscil/oscil.go)) instead. Where Go
generics map cleanly onto a template with no meaningful runtime cost --
`Line[T]`, `AudioDelay[T]` -- this port uses them.

## Wavetables

All 59 tables under `Mozzi/tables/*.h` (excluding the `BandLimited_SAW/SQUARE/TRI`
subdirectories, see below) were converted mechanically by
[`tools/gentables`](tools/gentables) into Go source backed by `//go:embed`
binary blobs. This is *better* than upstream's PROGMEM/`pgm_read_byte`
dance, not just a port of it: the RP2040 (like every other Cortex-M target
TinyGo supports) addresses flash directly, so a plain embedded byte slice
already lives in flash and costs zero RAM -- confirmed by building with all
59 tables referenced at runtime-variable indices (i.e. in a way the
compiler can't just constant-fold away): flash grows by the tables' actual
size, RAM does not grow at all.

Table names follow the array name from the original header
(`SIN2048_DATA` -> `tables.Sin2048`, `WAVESHAPE_SIGMOID_DATA` ->
`tables.WaveshapeSigmoid`, etc). See [`tables/doc.go`](tables/doc.go) for
the full generated list.

**Not included by default**: the `BandLimited_SAW/SQUARE/TRI` table sets
(multiple resolutions each, for anti-aliased `MetaOscil`-style playback) --
together they're ~4MB, dwarfing the rest of the table set, and this port
doesn't include `MetaOscil` or `AutoMap` to make use of them anyway.
Regenerate them yourself if you need them:

```sh
go run ./tools/gentables ../Mozzi/tables/BandLimited_SAW tables/bandlimited
```

(the generator works on any directory of `CONSTTABLE_STORAGE(...)` headers, not just the ones in `Mozzi/tables`)

## Playing a melody

[`song`](song) is a small addition with no upstream Mozzi equivalent: a
monophonic melody sequencer for "play this piece of music" sketches. A tune
is a `[]song.Event{Note, Beats}` slice -- `song.Note` constants (`C4` through
`B6`, i.e. named the way notes are named on a staff) plus `song.Rest` for
silence -- played back by a `song.Player` against an `oscil.Oscil`:

```go
var melody = []song.Event{
    {song.G4, 1}, {song.C5, 1}, {song.E5, 1}, {song.G5, 2},
    {song.Rest, 0.5},
    {song.E5, 1}, {song.G5, 3},
}

// in your sketch:
player := song.NewPlayer(osc, controlRate, bpm, melody, true) // true: loop
func (s *sketch) UpdateControl() { s.player.Update() }
func (s *sketch) UpdateAudio() audioout.Mono {
    if !s.player.Playing() { return audioout.From8Bit(0) } // silence during a Rest
    return audioout.From8Bit(int16(s.osc.Next()))
}
```

`Beats` is in quarter-note units, independent of `bpm` (tempo in quarter
notes/minute) -- retiming a whole piece is a one-constant change, not a
rewrite of every event. See [`examples/TinyGo_Extras/fanfare`](examples/TinyGo_Extras/fanfare) for a
complete sketch, including the workflow that produced its `melody`: extract
audio from a reference recording (`ffmpeg -vn`), then pitch- and
onset-track it (a small hand-rolled numpy autocorrelation tracker + energy-
envelope onset detector -- no scipy/librosa/aubio needed) to get measured,
not estimated, notes and durations. [`examples/TinyGo_Extras/furuhata_op`](examples/TinyGo_Extras/furuhata_op)
shows the other end of that spectrum: a melody transcribed from a
notation-software PDF export instead of audio, which reads back too
scrambled to trust beyond its clearest, most-repeated phrase -- its doc
comment is explicit about exactly how much of the piece that covers, rather
than filling the gap with confident-looking guesses.

## What's not ported

Kept out of scope, to keep this a manageable, coherent core rather than a
sprawling half-finished transliteration. Each of these follows the same
patterns already established (a plain-field port of the C++ class, or a
generated table, or a new `engine.Backend`), so they're reasonable places
to extend from:

- **Other hardware backends** (AVR, SAMD, STM32, Teensy, esp8266, classic
  ESP32, ...) -- see the [Boards](#boards) table above for what's shipped,
  what's likely feasible, and what TinyGo itself doesn't support yet.
  `engine.Backend` is the extension point.
- **Most sensor/analog input helpers** (`RCpoll.h`, `CapPoll.h`,
  `AutoRange.h`, `AutoMap.h`, `IntMap.h`, `RollingStat.h`, `Portamento.h`,
  `DCfilter.h`), audio input, and the `03.Sensors`/`04.Audio_Input` example
  families. `mozziAnalogRead` (as [`analog`](analog)) and `RollingAverage.h`
  are ported.
- **Most sample playback** (`Sample.h`, the other `samples/*.h` audio data,
  `08.Samples` examples) and **`MetaOscil`/`WavePacket`/`OverSample`**.
  `SampleHuffman.h` and the thumb piano samples are ported.
- **`AudioDelayFeedback`'s ALLPASS interpolation mode** and its int16-cell
  template variant -- only the (default, more common) int8-cell LINEAR mode
  is ported.
- **`StateVariable.h`, `PDResonant.h`, `WaveFolder.h`, `WaveShaper.h`,
  `ReverbTank.h`** and the corresponding example families.
- **Examples beyond the ones listed in [Examples](#examples)** -- the rest
  of upstream's `03.Sensors` to `13.External_Audio_Output`.
- **MIDI/serial communication** (`11.Communication` examples).
- **The newer `FixMath` library** (`UFix`/`SFix` type-safe fixed point
  templates that some newer upstream examples use instead of the classic
  `mozzi_fixmath.h` typedefs). This port sticks to the classic API, which
  covers everything the ported code needs. `mozzi/fixmath` is the classic-API
  equivalent.
- **Stereo output** (`AudioOutput`/`StereoOutput` exists in `audioout`, but
  no example or backend wiring for it yet).

## A note on fidelity

Most of this port is a direct, line-by-line translation of the upstream
algorithm -- same shifts, same fixed-point formats, same formulas. A few
places deliberately diverge, always in the direction of *more* correctness
or robustness, and are called out in comments where they matter:

- `filter.Filter8`/`Filter16` use one consistent integer width (int32
  state, int64 products) rather than replicating upstream's C++
  `IntegerType<>`-selected per-subexpression widths, which have no
  algorithmic meaning of their own.
- `midi.Q16n16MtoF` clamps its table lookup instead of reading one entry
  past the end of the table (harmless on a flat C++ address space; a Go
  slice-bounds panic otherwise).
- `audioout.Bits` fixes the logical output resolution at 8 bits for every
  backend, rather than upstream's per-platform-configurable
  `MOZZI_AUDIO_BITS`; see the package doc for why. So upstream's `_HIFI`
  examples, whose point is AVR's ~14 bit two-pin PWM mode, play here like
  their plain versions.
- `eventdelay.EventDelay` and `metronome.Metronome` convert their delay
  from milliseconds to audio ticks when started rather than when set,
  because the audio rate is a constant upstream but only known here once
  `engine.Start` has run -- so they can be created before it.

## Extending

- **New backend**: implement `engine.Backend`'s one method, `Start(sampleRateHz uint32, pull func() audioout.Storage)`,
  calling `pull()` from whatever periodic interrupt/DMA-complete callback
  (or, as in `rp2040i2s`, a goroutine paced by a blocking hardware write)
  your hardware provides, and write the sample to your DAC/PWM/I2S output.
  `backend/rp2040pwm` and `backend/esp32s3pwm` (~150 lines each, timer
  interrupt-driven), `backend/esp32s3i2s` (DMA-ring, refilled from the
  DMA-complete interrupt) and `backend/rp2040i2s` (~60 lines,
  goroutine-driven) are reasonable templates depending on which style your
  hardware suits --
  `esp32c3` in particular should be a light edit of `esp32s3pwm` (same LEDC
  PWM API; you'd need to find a free CPU interrupt line and TIMG timer the
  way its package doc explains).
- **New board**: add a `board/board_<name>.go` file behind its own build
  tag, defining `NewOutput`, `A0`, `Button` and `applyVolume` the way the
  existing ones do (and exclude the tag in `board_unselected.go` if it's
  for a target that already has a default).
- **New wavetable**: drop a Mozzi-format header (or any file with a
  `CONSTTABLE_STORAGE(type) NAME[] = {...}` array) anywhere, and run
  `go run ./tools/gentables <dir> <output-package-dir>`.
- **New Huffman-compressed sample**: run `go run ./tools/genhuffman <dir of
  audio2huff.py headers> <output-package-dir>` (as was done for
  `samples/thumbpianohuffman`).
- **New example**: copy `examples/01.Basics/Skeleton` (or, for a melody
  player, `examples/TinyGo_Extras/fanfare`), into the folder matching
  upstream's if it's a port of an upstream example.

## Verification status

Everything in this repo has been checked with `tinygo build`/`-size` (and,
for the hardware-independent packages, `go build`/`go vet`/`go run` against
extra generic instantiations beyond what the examples exercise). `rp2040i2s`
has been tried on real hardware (a Waveshare RP2040-Zero + MAX98357A). Its
first version produced no sound at all -- not a wiring problem, but a real
bug: its output loop runs in a goroutine, and TinyGo's cooperative
scheduler never ran it, because `for { engine.AudioHook() }` (every
example's main loop) never yielded. Fixed once, centrally, by having
`engine.AudioHook()` call `runtime.Gosched()` on every call (see its doc);
this is why *any* goroutine-driven `engine.Backend` needs nothing extra
from a sketch's main loop. After that fix, audio was confirmed audible on
that same board. `esp32s3pwm` has since been tried on real hardware too
(an M5Stack CoreS3-family board, TinyGo 0.42.0), and it did misbehave
there exactly the way a compile-clean register-level backend can: the
sample interrupt fired once and never again, because TIMG's `ALARM_EN`
bit clears itself on every alarm even with auto-reload on. Fixed by
re-arming it in the interrupt handler (see `isr()`); after that, an
on-chip self-test (no speaker or scope -- it reads the PWM pin's own pad
level back through the GPIO input register) measured a 16393Hz sample
interrupt (1MHz/61, i.e. 16384Hz nominal), `UpdateAudio` keeping pace with
it, `UpdateControl` at ~64Hz, and a pin duty cycle swinging 0-100% as a
~440Hz sine. Not yet confirmed by ear. `esp32s3i2s` was checked on that
same CoreS3 through its AW88298 speaker amp
([`examples/01.Basics/Sinewave`](examples/01.Basics/Sinewave), `-tags cores3`),
and is confirmed audible there: the GDMA interrupt pulled 16373-16384
samples/s, BCLK/WS/data toggled at ~32fs/fs, the amp's own status register
went from "no clock" to "PLL locked, clock stable" once I2S started, and
the 440Hz tone came out of the speaker. `rp2040pwm` has not been tried on
real hardware yet. Two more things are worth flagging specifically:

- `esp32s3pwm` pokes Xtensa interrupt-matrix and TIMG timer registers
  directly (worked out by reading TinyGo's own runtime source for the
  register/interrupt-line conventions it uses, since there's no
  higher-level "repeating timer" API to build on for that chip) -- the
  `ALARM_EN` bug above is the kind of thing that only real silicon shows.
- `rp2040i2s` builds on `github.com/tinygo-org/pio`'s `piolib.I2S`, an
  external, community-maintained PIO program this port did not write --
  reasonable to trust more than a from-scratch register-level backend, but
  still unverified *in this repo*, and this port's channel-duplication and
  8-to-16-bit scaling around it are new code.

Treat first flash as the actual test.

## License

Like upstream Mozzi, this port is licensed under the GNU Lesser General
Public License (LGPL) v2.1 or later -- see [`LICENSE.TXT`](LICENSE.TXT) /
[`LICENCE.LGPL`](LICENCE.LGPL).
