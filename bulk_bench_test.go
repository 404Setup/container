package container_test

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/404Setup/container"
)

func randomEntries(n int, seed uint64) []container.Entry[int, int] {
	rng := rand.New(rand.NewPCG(seed, seed))
	entries := make([]container.Entry[int, int], n)
	for i := range entries {
		entries[i] = container.Entry[int, int]{Key: rng.IntN(n * 4), Value: i}
	}
	return entries
}

func BenchmarkMapInit(b *testing.B) {
	for _, size := range []int{32, 1024, 65536} {
		entries := randomEntries(size, 1)
		b.Run("random/"+strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				containerBenchmarkSink = container.NewOrderedMap(entries...).Len()
			}
		})
		sorted := make([]container.Entry[int, int], size)
		for i := range sorted {
			sorted[i] = container.Entry[int, int]{Key: i, Value: i}
		}
		b.Run("sorted/"+strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				containerBenchmarkSink = container.NewOrderedMap(sorted...).Len()
			}
		})
	}
}

func BenchmarkMapStoreAll(b *testing.B) {
	for _, tc := range []struct{ existing, batch int }{{1024, 16}, {1024, 1024}, {65536, 64}, {65536, 8192}} {
		base := container.NewOrderedMap(randomEntries(tc.existing, 2)...)
		batch := randomEntries(tc.batch, 3)
		b.Run(strconv.Itoa(tc.existing)+"+"+strconv.Itoa(tc.batch), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				m := base.Clone()
				m.Grow(tc.batch)
				b.StartTimer()
				m.StoreAll(batch...)
				containerBenchmarkSink = m.Len()
			}
		})
	}
}

func BenchmarkSetAddAll(b *testing.B) {
	for _, size := range []int{32, 1024, 65536} {
		values := make([]int, size)
		for i, e := range randomEntries(size, 4) {
			values[i] = e.Key
		}
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				containerBenchmarkSink = container.NewOrderedSet(values...).Len()
			}
		})
	}
}

func BenchmarkPriorityQueuePushAll(b *testing.B) {
	values := make([]int, 4096)
	for i := range values {
		values[i] = rand.IntN(1 << 20)
	}
	cmpInt := func(a, b int) int { return a - b }
	b.Run("all", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			q := container.NewPriorityQueue(cmpInt)
			q.PushAll(values...)
			containerBenchmarkSink = q.Len()
		}
	})
	b.Run("one-by-one", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			q := container.NewPriorityQueue(cmpInt)
			for _, v := range values {
				q.Push(v)
			}
			containerBenchmarkSink = q.Len()
		}
	})
}

func BenchmarkDequePushBackAll(b *testing.B) {
	values := make([]int, 1024)
	b.Run("all", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var d container.Deque[int]
			d.PushBackAll(values...)
			containerBenchmarkSink = d.Len()
		}
	})
	b.Run("one-by-one", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var d container.Deque[int]
			for _, v := range values {
				d.PushBack(v)
			}
			containerBenchmarkSink = d.Len()
		}
	})
}
