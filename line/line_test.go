package line

import "testing"

// ramp runs numSteps steps of a Line from start to target and returns every value.
func ramp[T Number](start, target, numSteps T) []T {
	l := New[T]()
	l.SetFromTo(start, target, numSteps)
	out := make([]T, 0, int(numSteps))
	for i := T(0); i < numSteps; i++ {
		out = append(out, l.Next())
	}
	return out
}

func checkRamp[T Number](t *testing.T, name string, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d values, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestUnsignedRampDown(t *testing.T) {
	checkRamp(t, "uint8 200->100", ramp[uint8](200, 100, 10),
		[]uint8{190, 180, 170, 160, 150, 140, 130, 120, 110, 100})
	checkRamp(t, "uint16 60000->0", ramp[uint16](60000, 0, 3),
		[]uint16{40000, 20000, 0})
	checkRamp(t, "uint32 1000->0", ramp[uint32](1000, 0, 4),
		[]uint32{750, 500, 250, 0})
	checkRamp(t, "uint64 1<<40->0", ramp[uint64](1<<40, 0, 2),
		[]uint64{1 << 39, 0})
}

func TestUnsignedRampDownTruncatesLikeUpstream(t *testing.T) {
	// Upstream truncates ((float)target-current)/num_steps toward zero: -105/10 -> -10.
	checkRamp(t, "uint8 205->100", ramp[uint8](205, 100, 10),
		[]uint8{195, 185, 175, 165, 155, 145, 135, 125, 115, 105})
}

func TestUnsignedRampUp(t *testing.T) {
	checkRamp(t, "uint8 100->200", ramp[uint8](100, 200, 10),
		[]uint8{110, 120, 130, 140, 150, 160, 170, 180, 190, 200})
	checkRamp(t, "uint32 0->1000", ramp[uint32](0, 1000, 4),
		[]uint32{250, 500, 750, 1000})
}

func TestSignedAndFloatRamps(t *testing.T) {
	// (target-current must fit in T for signed types, same as upstream's generic Line<T>.)
	checkRamp(t, "int8 60->-60", ramp[int8](60, -60, 4),
		[]int8{30, 0, -30, -60})
	checkRamp(t, "int32 -1000->1000", ramp[int32](-1000, 1000, 4),
		[]int32{-500, 0, 500, 1000})
	checkRamp(t, "float32 1->0", ramp[float32](1, 0, 4),
		[]float32{0.75, 0.5, 0.25, 0})
}

func TestZeroStepsJumpsToTarget(t *testing.T) {
	l := New[uint8]()
	l.Set(200)
	l.SetTarget(100, 0)
	if got := l.Next(); got != 100 {
		t.Fatalf("got %d, want 100", got)
	}
}
