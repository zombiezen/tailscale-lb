// Package deque provides a double-ended queue.
package deque

import (
	"fmt"
	"math/bits"
)

const initialCapacity = 8

// Deque is a double-ended queue.
// The zero value is an empty deque.
type Deque[T any] struct {
	array []T
	front int
	back  int // exclusive
}

// Len returns the number of elements in the deque.
func (d *Deque[T]) Len() int {
	return count(d.front, d.back, len(d.array))
}

func (d *Deque[T]) isFull() bool {
	return len(d.array)-d.Len() <= 1
}

// At returns the element at the given index,
// with 0 being the front of the queue.
// Panics if i is negative or greater than or equal to d.Len().
func (d *Deque[T]) At(i int) T {
	if n := d.Len(); i < 0 || i >= n {
		panic(fmt.Errorf("deque index %d out of range %d", i, n))
	}
	return d.array[wrapIndex(d.front+i, len(d.array))]
}

// Front returns the element at the front of the queue.
func (d *Deque[T]) Front() (_ T, ok bool) {
	var x T
	if d.front == d.back {
		return x, false
	}
	x = d.array[d.front]
	return x, true
}

// PopFront removes the element at the front of the queue and returns it.
func (d *Deque[T]) PopFront() (_ T, ok bool) {
	x, ok := d.Front()
	if ok {
		var zero T
		d.array[d.front] = zero
		d.front = wrapIndex(d.front+1, len(d.array))
	}
	return x, ok
}

// Append inserts an element at the back of the queue.
func (d *Deque[T]) Append(x T) {
	if d.isFull() {
		d.grow()
	}
	d.array[d.back] = x
	d.back = wrapIndex(d.back+1, len(d.array))
}

func (d *Deque[T]) Filter(pred func(T) bool) {
	if d.front == d.back {
		return
	}
	oldLen := d.Len()
	n := 0
	for i, end := d.front, d.front+oldLen; i < end; i++ {
		x := d.array[wrapIndex(i, len(d.array))]
		if pred(x) {
			d.array[wrapIndex(d.front+n, len(d.array))] = x
			n++
		}
	}
	clearStart := wrapIndex(d.front+n, len(d.array))
	if clearStart <= d.back {
		clear(d.array[clearStart:d.back])
	} else {
		clear(d.array[clearStart:])
		clear(d.array[:d.back])
	}
	d.back = wrapIndex(d.back+n-oldLen, len(d.array))
}

// Rotate rotates the deque n places to the left,
// such that the n'th item will be at the front of the deque.
// Negative values rotate the deque to the right.
func (d *Deque[T]) Rotate(n int) {
	if d.front == d.back {
		return
	}
	len := d.Len()
	n = wrapIndex(n, len)
	if n == 0 {
		return
	}
	k := len - n
	if n <= k {
		d.rotateLeft(n)
	} else {
		d.rotateRight(k)
	}
}

func (d *Deque[T]) rotateLeft(mid int) {
	wrapCopy(d.array, d.back, d.front, mid)
	d.front = wrapIndex(d.front+mid, len(d.array))
	d.back = wrapIndex(d.back+mid, len(d.array))
}

func (d *Deque[T]) rotateRight(k int) {
	d.front = wrapIndex(d.front-k, len(d.array))
	d.back = wrapIndex(d.back-k, len(d.array))
	wrapCopy(d.array, d.front, d.back, k)
}

// wrapCopy copies a potentially wrapping block of memory n long from src to dst.
// abs(dst - src) + n must be no larger than len(array)
// (i.e. there must be at most one continous overlapping region between src and dst).
func wrapCopy[T any](array []T, dst, src, n int) {
	if src == dst || n == 0 {
		return
	}
	dstAfterSrc := wrapIndex(dst-src, len(array)) < n
	srcPreWrapLen := len(array) - src
	dstPreWrapLen := len(array) - dst
	srcWraps := srcPreWrapLen < n
	dstWraps := dstPreWrapLen < n

	switch {
	case !srcWraps && !dstWraps:
		copy(array[dst:], array[src:src+n])
	case !dstAfterSrc && !srcWraps && dstWraps:
		copy(array[dst:], array[src:src+dstPreWrapLen])
		copy(array, array[src+dstPreWrapLen:src+n])
	case dstAfterSrc && !srcWraps && dstWraps:
		copy(array, array[src+dstPreWrapLen:src+n])
		copy(array[dst:], array[src:src+dstPreWrapLen])
	case !dstAfterSrc && srcWraps && !dstWraps:
		copy(array[dst:], array[src:src+srcPreWrapLen])
		copy(array[dst+srcPreWrapLen:], array[:n-srcPreWrapLen])
	case dstAfterSrc && srcWraps && !dstWraps:
		copy(array[dst+srcPreWrapLen:], array[:n-srcPreWrapLen])
		copy(array[dst:], array[src:src+srcPreWrapLen])
	case !dstAfterSrc && srcWraps && dstWraps:
		delta := dstPreWrapLen - srcPreWrapLen
		copy(array[dst:], array[src:src+srcPreWrapLen])
		copy(array[dst+srcPreWrapLen:], array[:delta])
		copy(array, array[delta:delta+n-dstPreWrapLen])
	default:
		delta := srcPreWrapLen - dstPreWrapLen
		copy(array[delta:], array[:n-srcPreWrapLen])
		copy(array, array[len(array)-delta:])
		copy(array[dst:], array[src:src+dstPreWrapLen])
	}
}

func (d *Deque[T]) grow() {
	var newCap int
	if len(d.array) < initialCapacity {
		newCap = initialCapacity
	} else {
		newCap = len(d.array) * 2
	}
	newArray := make([]T, newCap)
	oldLen := d.Len()
	if d.front <= d.back {
		copy(newArray, d.array[d.front:d.back])
	} else {
		split := copy(newArray, d.array[d.front:])
		copy(newArray[split:], d.array[:d.back])
	}
	d.array = newArray
	d.front = 0
	d.back = oldLen
}

func clear[T any](s []T) {
	var zero T
	for i := range s {
		s[i] = zero
	}
}

func count(front, back, size int) int {
	return int((uint(back) - uint(front)) & uint(size-1))
}

func wrapIndex(i, size int) int {
	if !isPowerOfTwo(size) {
		for i < 0 {
			i += size
		}
		return i % size
	}
	return i & (size - 1)
}

func isPowerOfTwo(n int) bool {
	return n > 0 && bits.OnesCount(uint(n)) == 1
}
