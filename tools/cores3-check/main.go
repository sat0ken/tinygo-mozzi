// Command cores3-check checks the M5Stack CoreS3's audio path, step by
// step, while playing a 440Hz test tone through the built-in speaker: is
// the engine producing samples at the audio rate, at what level, is I2S
// data actually reaching the amp, and is the amp powered, enabled and
// locked to the I2S clock. It prints a report over USB serial every
// second, so you can tell "nothing is coming out of the board" or "the amp
// is clipping" (heard as crackling) apart from "the sketch isn't doing what
// I expected".
//
// It's a TinyGo program for the board, not a host tool:
//
//	tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./tools/cores3-check
//	tinygo monitor -port /dev/ttyACM0
//
// Like the examples, it honours -ldflags="-X mozzi/board.volume=N".
package main

import (
	"time"

	"mozzi/audioout"
	"mozzi/board"
	"mozzi/board/cores3"
	"mozzi/engine"
	"mozzi/oscil"
	"mozzi/tables"
)

const (
	audioRate   = 16384
	controlRate = 64
)

type sketch struct {
	aSin *oscil.Oscil
	peak int32
}

func (s *sketch) UpdateControl() {}

func (s *sketch) UpdateAudio() audioout.Mono {
	m := audioout.From8Bit(int16(s.aSin.Next()))
	if v := m.L(); v > s.peak {
		s.peak = v
	}
	return m
}

func verdict(ok bool) string {
	if ok {
		return "OK"
	}
	return "NG"
}

// doutEdges counts level changes on the speaker's I2S data pin per second:
// zero means the amp is being sent nothing but silence.
func doutEdges() int64 {
	pin := cores3.SpeakerDOUT
	t0 := time.Now()
	prev, edges := pin.Get(), 0
	for i := 0; i < 50000; i++ { // a few ms; the DMA ring and engine buffer cover it
		if v := pin.Get(); v != prev {
			edges++
			prev = v
		}
	}
	return int64(edges) * 1_000_000 / time.Since(t0).Microseconds()
}

func main() {
	s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
	s.aSin.SetFreq(440)

	out, err := board.NewOutput(audioRate)
	for err != nil {
		println("cores3-check: setting up the speaker failed:", err.Error())
		time.Sleep(time.Second)
	}
	engine.Start(s, audioRate, controlRate, out)

	for {
		t0, k0 := time.Now(), engine.AudioTicks()
		s.peak = 0
		// Sample the amp's status 10 times over the second: clipping and the
		// output stage dropping out come and go.
		var st cores3.SpeakerStatus
		reads, clipping, notSwitching := 0, 0, 0
		next := t0
		for time.Since(t0) < time.Second {
			engine.AudioHook()
			if time.Now().Before(next) {
				continue
			}
			next = next.Add(100 * time.Millisecond)
			st = cores3.ReadSpeakerStatus()
			if st.Err != nil {
				break
			}
			reads++
			if st.Clipping {
				clipping++
			}
			if !st.Switching {
				notSwitching++
			}
		}
		rate := int64(engine.AudioTicks()-k0) * 1_000_000 / time.Since(t0).Microseconds()
		edges := doutEdges()

		println("---- cores3-check: playing a 440Hz test tone ----")
		println("audio out  :", rate, "samples/s (expect ~", audioRate, ")", verdict(rate > audioRate*95/100 && rate < audioRate*105/100))
		println("tone level :", s.peak, "of", audioout.Bias-1, "at volume", board.Volume(), "of 255", verdict(s.peak > 0 && board.Volume() > 0))
		println("I2S data   :", edges, "edges/s on DOUT (G13)", verdict(edges > 1000))
		if st.Err != nil {
			println("amp        : can't read it over I2C:", st.Err.Error(), "NG")
			continue
		}
		println("amp supply : AXP2101 ALDO1", onOff(st.SupplyOn), verdict(st.SupplyOn))
		println("amp        : enabled", yesNo(st.Enabled), "| PLL locked", yesNo(st.PLLLocked),
			"| clock stable", yesNo(st.ClockOK), "| no clock", yesNo(st.NoClock), verdict(st.Enabled && !st.NoClock))
		println("amp output : clipping", clipping, "of", reads, "reads | output stage stopped", notSwitching, "of", reads, verdict(clipping == 0 && notSwitching == 0))
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
