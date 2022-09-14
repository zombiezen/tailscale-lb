package deque

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestZero(t *testing.T) {
	d := new(Deque[int])
	if got := d.Len(); got != 0 {
		t.Errorf("d.Len() = %d; want 0", got)
	}
	if got, ok := d.Front(); ok {
		t.Errorf("d.Front() = %v, true; want _, false", got)
	}
	if got, ok := d.PopFront(); ok {
		t.Errorf("d.PopFront() = %v, true; want _, false", got)
	}
}

func TestAtPanic(t *testing.T) {
	t.Run("Negative", func(t *testing.T) {
		d := new(Deque[int])
		d.Append(1)

		defer func() {
			if err := recover(); err == nil {
				t.Error("At did not panic")
			}
		}()
		d.At(-1)
	})

	t.Run("PastLen", func(t *testing.T) {
		d := new(Deque[int])
		d.Append(1)

		defer func() {
			if err := recover(); err == nil {
				t.Error("At did not panic")
			}
		}()
		d.At(1)
	})
}

func TestAppend(t *testing.T) {
	d := new(Deque[int])

	d.Append(10)
	if diff := cmp.Diff([]int{10}, toSlice(d)); diff != "" {
		t.Errorf("after first append (-want +got):\n%s", diff)
	}
	if got, ok := d.Front(); !ok || got != 10 {
		t.Errorf("d.Front() = %d, %t; want 10, true", got, ok)
	}

	d.Append(11)
	if diff := cmp.Diff([]int{10, 11}, toSlice(d)); diff != "" {
		t.Errorf("after second append (-want +got):\n%s", diff)
	}
	if got, ok := d.Front(); !ok || got != 10 {
		t.Errorf("d.Front() = %d, %t; want 10, true", got, ok)
	}

	d.Append(12)
	if diff := cmp.Diff([]int{10, 11, 12}, toSlice(d)); diff != "" {
		t.Errorf("after third append (-want +got):\n%s", diff)
	}
	if got, ok := d.Front(); !ok || got != 10 {
		t.Errorf("d.Front() = %d, %t; want 10, true", got, ok)
	}
}

func TestPopFront(t *testing.T) {
	d := new(Deque[int])
	d.Append(10)
	d.Append(11)
	d.Append(12)

	if got, ok := d.PopFront(); !ok || got != 10 {
		t.Errorf("first d.PopFront() = %d, %t; want 10, true", got, ok)
	}
	if got, want := d.Len(), 2; got != want {
		t.Errorf("after first pop, d.Len() = %d; want %d", got, want)
	}

	if got, ok := d.PopFront(); !ok || got != 11 {
		t.Errorf("second d.PopFront() = %d, %t; want 11, true", got, ok)
	}
	if got, want := d.Len(), 1; got != want {
		t.Errorf("after second pop, d.Len() = %d; want %d", got, want)
	}

	if got, ok := d.PopFront(); !ok || got != 12 {
		t.Errorf("third d.PopFront() = %d, %t; want 12, true", got, ok)
	}
	if got, want := d.Len(), 0; got != want {
		t.Errorf("after third pop, d.Len() = %d; want %d", got, want)
	}

	if got, ok := d.PopFront(); ok {
		t.Errorf("fourth d.PopFront() = %d, %t; want _, false", got, ok)
	}
	if got, want := d.Len(), 0; got != want {
		t.Errorf("after fourth pop, d.Len() = %d; want %d", got, want)
	}
}

func TestGrowContiguous(t *testing.T) {
	d := new(Deque[int])
	counter := 0
	for i := 0; i < initialCapacity; i++ {
		d.Append(counter)
		counter++
	}
	if len(d.array) <= initialCapacity {
		t.Errorf("len(d.array) = %d; want >%d", len(d.array), initialCapacity)
	}
	if diff := cmp.Diff(seq(0, initialCapacity), toSlice(d), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("after growing (-want +got):\n%s", diff)
	}
}

func TestGrowSplit(t *testing.T) {
	d := new(Deque[int])
	counter := 0
	for i := 0; i < initialCapacity-1; i++ {
		d.Append(counter)
		counter++
	}
	for i := 0; i < 3; i++ {
		d.PopFront()
	}
	for i := 0; i < 3; i++ {
		d.Append(counter)
		counter++
	}
	if diff := cmp.Diff(seq(3, initialCapacity+2), toSlice(d), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("before growing (-want +got):\n%s", diff)
	}

	for i := 0; i < 3; i++ {
		d.Append(counter)
		counter++
	}
	if diff := cmp.Diff(seq(3, initialCapacity+5), toSlice(d), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("after growing (-want +got):\n%s", diff)
	}
}

func TestRotate(t *testing.T) {
	tests := []struct {
		init   []int
		rotate int
		want   []int
	}{
		{
			init:   seq(0, 10),
			rotate: 0,
			want:   seq(0, 10),
		},
		{
			init:   seq(0, 10),
			rotate: 10,
			want:   seq(0, 10),
		},
		{
			init:   seq(0, 10),
			rotate: 3,
			want:   append(seq(3, 10), seq(0, 3)...),
		},
		{
			init:   seq(0, 10),
			rotate: -7,
			want:   append(seq(3, 10), seq(0, 3)...),
		},
		{
			init:   seq(0, 10),
			rotate: -3,
			want:   append(seq(7, 10), seq(0, 7)...),
		},
		{
			init:   seq(0, 10),
			rotate: 7,
			want:   append(seq(7, 10), seq(0, 7)...),
		},
	}
	for _, test := range tests {
		d := new(Deque[int])
		for _, x := range test.init {
			d.Append(x)
		}
		d.Rotate(test.rotate)
		got := toSlice(d)
		if !cmp.Equal(test.want, got, cmpopts.EquateEmpty()) {
			t.Errorf("Deque%v.Rotate(%d) = %v; want %v", test.init, test.rotate, got, test.want)
		}
	}
}

func TestWrapCopy(t *testing.T) {
	tests := []struct {
		array []int
		dst   int
		src   int
		n     int
		want  []int
	}{
		{
			array: seq(0, 5),
			src:   0,
			dst:   0,
			n:     0,
			want:  seq(0, 5),
		},
		{
			array: seq(0, 9),
			src:   2,
			dst:   4,
			n:     4,
			want:  []int{0, 1, 2, 3, 2, 3, 4, 5, 8},
		},
		{
			array: seq(0, 9),
			src:   0,
			dst:   7,
			n:     4,
			want:  []int{2, 3, 2, 3, 4, 5, 6, 0, 1},
		},
		{
			array: seq(0, 9),
			src:   5,
			dst:   7,
			n:     4,
			want:  []int{7, 8, 2, 3, 4, 5, 6, 5, 6},
		},
		{
			array: seq(0, 9),
			src:   7,
			dst:   5,
			n:     4,
			want:  []int{0, 1, 2, 3, 4, 7, 8, 0, 1},
		},
		{
			array: seq(0, 9),
			src:   7,
			dst:   0,
			n:     4,
			want:  []int{7, 8, 0, 1, 4, 5, 6, 7, 8},
		},
		{
			array: seq(0, 9),
			src:   7,
			dst:   6,
			n:     5,
			want:  []int{1, 2, 2, 3, 4, 5, 7, 8, 0},
		},
		{
			array: seq(0, 9),
			src:   6,
			dst:   7,
			n:     5,
			want:  []int{8, 0, 1, 3, 4, 5, 6, 6, 7},
		},
	}
	for _, test := range tests {
		array := append([]int(nil), test.array...)
		wrapCopy(array, test.dst, test.src, test.n)
		if !cmp.Equal(test.want, array) {
			t.Errorf("wrapCopy(%v, %d, %d, %d) = %v; want %v", test.array, test.dst, test.src, test.n, array, test.want)
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		front int
		back  int
		size  int
		want  int
	}{
		{0, 0, 8, 0},
		{0, 1, 8, 1},
		{7, 0, 8, 1},
	}
	for _, test := range tests {
		if got := count(test.front, test.back, test.size); got != test.want {
			t.Errorf("count(%d, %d, %d) = %d; want %d", test.front, test.back, test.size, got, test.want)
		}
	}
}

func TestWrapIndex(t *testing.T) {
	tests := []struct {
		i    int
		size int
		want int
	}{
		{-4, 4, 0},
		{-3, 4, 1},
		{-2, 4, 2},
		{-1, 4, 3},
		{0, 4, 0},
		{1, 4, 1},
		{2, 4, 2},
		{3, 4, 3},
		{4, 4, 0},
		{5, 4, 1},
		{6, 4, 2},
		{7, 4, 3},

		// Non-power-of-two
		{-5, 5, 0},
		{-4, 5, 1},
		{-3, 5, 2},
		{-2, 5, 3},
		{-1, 5, 4},
		{0, 5, 0},
		{1, 5, 1},
		{2, 5, 2},
		{3, 5, 3},
		{4, 5, 4},
		{5, 5, 0},
		{6, 5, 1},
		{7, 5, 2},
		{8, 5, 3},
		{9, 5, 4},
		{10, 5, 0},
	}
	for _, test := range tests {
		if got := wrapIndex(test.i, test.size); got != test.want {
			t.Errorf("wrapIndex(%d, %d) = %d; want %d", test.i, test.size, got, test.want)
		}
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{6, false},
		{7, false},
		{8, true},
	}

	for _, test := range tests {
		if got := isPowerOfTwo(test.n); got != test.want {
			t.Errorf("isPowerOfTwo(%d) = %t; want %t", test.n, got, test.want)
		}
	}
}

func toSlice[T any](d *Deque[T]) []T {
	s := make([]T, d.Len())
	for i := range s {
		s[i] = d.At(i)
	}
	return s
}

func seq(start, end int) []int {
	s := make([]int, end-start)
	for i := range s {
		s[i] = start + i
	}
	return s
}

func TestSeq(t *testing.T) {
	tests := []struct {
		start int
		end   int
		want  []int
	}{
		{0, 0, []int{}},
		{0, 5, []int{0, 1, 2, 3, 4}},
		{5, 10, []int{5, 6, 7, 8, 9}},
	}
	for _, test := range tests {
		got := seq(test.start, test.end)
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("seq(%d, %d) (-want +got):\n%s", test.start, test.end, diff)
		}
	}
}
