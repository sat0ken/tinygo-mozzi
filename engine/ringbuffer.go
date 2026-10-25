package engine

// ringBuffer is a port of Mozzi's CircularBuffer.h, fixed at 256 cells
// (upstream's size-256 specialization, chosen because it lets start/end
// indices wrap for free via normal uint8 overflow, avoiding a modulo
// compare on every read/write) and generic over the stored item type in
// place of upstream's ITEM_TYPE template parameter.
//
// isFull/isEmpty distinguish "empty" from "full" (both of which have
// start==end) using one extra bit each (sMSB/eMSB) that flips every time
// its index wraps around -- see the "Mirroring version" note in the
// upstream header.
type ringBuffer[T any] struct {
	items          [256]T
	start, end     uint8
	sMSB, eMSB     uint8
	numBuffersRead uint32
}

func (b *ringBuffer[T]) isFull() bool  { return b.end == b.start && b.eMSB != b.sMSB }
func (b *ringBuffer[T]) isEmpty() bool { return b.end == b.start && b.eMSB == b.sMSB }

func (b *ringBuffer[T]) write(v T) {
	b.items[b.end] = v
	b.end++
	if b.end == 0 {
		b.eMSB ^= 1
	}
}

func (b *ringBuffer[T]) read() T {
	v := b.items[b.start]
	b.start++
	if b.start == 0 {
		b.sMSB ^= 1
		b.numBuffersRead++
	}
	return v
}

// count returns the total number of items ever read from the buffer.
func (b *ringBuffer[T]) count() uint64 {
	return uint64(b.numBuffersRead)<<8 + uint64(b.start)
}
