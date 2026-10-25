package rand

import "testing"

const draws = 20000

func TestInt8StaysInRange(t *testing.T) {
	for _, r := range [][2]int8{{-100, 100}, {-128, 127}, {0, 127}, {-10, 10}, {50, 60}} {
		minval, maxval := r[0], r[1]
		lo, hi := maxval, minval
		for i := 0; i < draws; i++ {
			v := Int8(minval, maxval)
			if v < minval || v >= maxval {
				t.Fatalf("Int8(%d, %d) = %d, out of range", minval, maxval, v)
			}
			lo, hi = min(lo, v), max(hi, v)
		}
		// The whole range should actually be used, not just a corner of it.
		if lo != minval || hi != maxval-1 {
			t.Errorf("Int8(%d, %d) covered only [%d, %d]", minval, maxval, lo, hi)
		}
	}
}

func TestInt16StaysInRange(t *testing.T) {
	for _, r := range [][2]int16{{-20000, 20000}, {-32768, 32767}, {-10, 10}, {1000, 2000}} {
		minval, maxval := r[0], r[1]
		lo, hi := maxval, minval
		for i := 0; i < draws; i++ {
			v := Int16(minval, maxval)
			if v < minval || v >= maxval {
				t.Fatalf("Int16(%d, %d) = %d, out of range", minval, maxval, v)
			}
			lo, hi = min(lo, v), max(hi, v)
		}
		// Within 1% of each end: wide ranges can't be hit exactly in 20000 draws.
		slack := (int32(maxval)-int32(minval))/100 + 1
		if int32(lo)-int32(minval) > slack || int32(maxval-1)-int32(hi) > slack {
			t.Errorf("Int16(%d, %d) covered only [%d, %d]", minval, maxval, lo, hi)
		}
	}
}
