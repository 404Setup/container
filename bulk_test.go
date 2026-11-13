package container_test

import (
	"cmp"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/404Setup/container"
)

func TestMapInitAndStoreAll(t *testing.T) {
	type E = container.Entry[int, string]
	m := container.NewOrderedMap(E{3, "c"}, E{1, "a"}, E{3, "C"}, E{2, "b"})
	if got, want := slices.Collect(m.Keys()), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v; want %v", got, want)
	}
	if v, _ := m.Load(3); v != "C" {
		t.Fatalf("Load(3) = %q; want last duplicate %q", v, "C")
	}
	m.StoreAll(E{4, "d"}, E{1, "A"}, E{0, "z"}, E{4, "D"})
	got := map[int]string{}
	for k, v := range m.All() {
		got[k] = v
	}
	want := map[int]string{0: "z", 1: "A", 2: "b", 3: "C", 4: "D"}
	if !reflect.DeepEqual(got, want) || !slices.IsSorted(slices.Collect(m.Keys())) {
		t.Fatalf("after StoreAll = %v; want %v", got, want)
	}
	m.StoreAll()
	if m.Len() != 5 {
		t.Fatalf("Len = %d; want 5", m.Len())
	}
}

func TestMapStoreAllRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	m := container.NewOrderedMap[int, int]()
	want := map[int]int{}
	for range 200 {
		var batch []container.Entry[int, int]
		for range rng.IntN(20) {
			e := container.Entry[int, int]{Key: rng.IntN(100), Value: rng.Int()}
			batch = append(batch, e)
			want[e.Key] = e.Value
		}
		m.StoreAll(batch...)
		if m.Len() != len(want) {
			t.Fatalf("Len = %d; want %d", m.Len(), len(want))
		}
	}
	prev := -1
	for k, v := range m.All() {
		if k <= prev || want[k] != v {
			t.Fatalf("bad entry %d=%d (prev %d)", k, v, prev)
		}
		prev = k
	}
}

func TestSetInitAndAddAll(t *testing.T) {
	s := container.NewOrderedSet(5, 1, 3, 1)
	if got, want := slices.Collect(s.All()), []int{1, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("set = %v; want %v", got, want)
	}
	if n := s.AddAll(3, 4, 4, 0, 6); n != 3 {
		t.Fatalf("AddAll added %d; want 3", n)
	}
	if got, want := slices.Collect(s.All()), []int{0, 1, 3, 4, 5, 6}; !reflect.DeepEqual(got, want) {
		t.Fatalf("set = %v; want %v", got, want)
	}
	if n := s.AddAll(); n != 0 {
		t.Fatalf("AddAll() = %d; want 0", n)
	}
	s2 := container.NewSet(func(a, b string) int { return cmp.Compare(b, a) }, "a", "c", "b")
	if got, want := slices.Collect(s2.All()), []string{"c", "b", "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("set = %v; want %v", got, want)
	}
}

func TestSequenceBulkAdd(t *testing.T) {
	v := container.NewVector(1)
	v.AppendSeq(slices.Values([]int{2, 3}))
	v.Append(4, 5)
	if got, want := v.Slice(), []int{1, 2, 3, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Vector = %v; want %v", got, want)
	}

	var d container.Deque[int]
	d.PushBackAll(3, 4)
	d.PushFrontAll(1, 2)
	d.PushBackAll(5, 6, 7, 8, 9, 10, 11)
	d.PushFrontAll(-1, 0)
	d.PushBackAll()
	if got, want := d.Slice(), []int{-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deque = %v; want %v", got, want)
	}

	var q container.Queue[int]
	q.EnqueueAll(1, 2)
	q.EnqueueAll(3)
	if got, want := q.Slice(), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Queue = %v; want %v", got, want)
	}

	var s container.Stack[int]
	s.PushAll(1, 2, 3)
	if x, _ := s.Pop(); x != 3 {
		t.Fatalf("Stack Pop = %d; want 3", x)
	}
}

func TestPriorityQueuePushAll(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	q := container.NewPriorityQueue(cmp.Compare[int])
	var want []int
	for range 200 {
		batch := make([]int, rng.IntN(30))
		for i := range batch {
			batch[i] = rng.IntN(1000)
		}
		q.PushAll(batch...)
		want = append(want, batch...)
		if q.Len() != len(want) {
			t.Fatalf("Len = %d; want %d", q.Len(), len(want))
		}
		if rng.IntN(3) == 0 {
			slices.Sort(want)
			if got, _ := q.Pop(); got != want[0] {
				t.Fatalf("Pop = %d; want %d", got, want[0])
			}
			want = want[1:]
		}
	}
	slices.Sort(want)
	if got := q.Slice(); !slices.Equal(got, want) {
		t.Fatalf("drain mismatch")
	}
}
