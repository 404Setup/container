package container

import (
	"cmp"
	"iter"
	"math/bits"
	"slices"
	"sort"
)

// Compare orders two values. It returns a negative number when a sorts before
// b, zero when they are equivalent, and a positive number when a sorts after b.
type Compare[E any] func(a, b E) int

// Map is a non-concurrent map that iterates in key order. It uses binary search
// over contiguous storage, favoring compact storage, lookups, and iteration.
// Insertions and deletions may move later entries.
//
// Use NewMap or NewOrderedMap to initialize a Map before storing entries.
type Map[K, V any] struct {
	compare Compare[K]
	entries []mapEntry[K, V]
}

type mapEntry[K, V any] struct {
	key   K
	value V
}

// Entry is a key-value pair used to initialize a Map or to store entries in
// bulk.
type Entry[K, V any] struct {
	Key   K
	Value V
}

// NewMap returns a Map ordered by compare, initialized with entries. If
// entries contains duplicate keys, the last one wins. Compare must define a
// strict total order and must not be nil.
func NewMap[K, V any](compare Compare[K], entries ...Entry[K, V]) *Map[K, V] {
	if compare == nil {
		panic("container: nil Map comparison")
	}
	m := &Map[K, V]{compare: compare}
	m.StoreAll(entries...)
	return m
}

// NewOrderedMap returns a Map ordered by cmp.Compare, initialized with
// entries. If entries contains duplicate keys, the last one wins.
func NewOrderedMap[K cmp.Ordered, V any](entries ...Entry[K, V]) *Map[K, V] {
	return NewMap[K, V](cmp.Compare[K], entries...)
}

// Len returns the number of entries in m.
func (m *Map[K, V]) Len() int { return len(m.entries) }

// Cap returns the capacity of m's entry storage.
func (m *Map[K, V]) Cap() int { return cap(m.entries) }

// Load returns the value stored for key. The ok result reports whether key was
// present.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	if len(m.entries) == 0 {
		return value, false
	}
	i, ok := m.search(key)
	if !ok {
		return value, false
	}
	return m.entries[i].value, true
}

// Store sets the value for key.
func (m *Map[K, V]) Store(key K, value V) {
	_, _ = m.Swap(key, value)
}

// Swap sets the value for key and returns the previous value, if any. The
// loaded result reports whether key was already present.
func (m *Map[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	i, loaded := m.search(key)
	if loaded {
		previous = m.entries[i].value
		m.entries[i].value = value
		return previous, true
	}
	m.entries = append(m.entries, mapEntry[K, V]{})
	copy(m.entries[i+1:], m.entries[i:])
	m.entries[i] = mapEntry[K, V]{key: key, value: value}
	return previous, false
}

// StoreAll sets the value for every entry in entries, as if by calling Store
// for each one in order: if a key appears more than once, the last value wins.
// It sorts the batch (skipping the sort if it is already in order) and merges
// it into m, so it avoids the O(k*n) cost of repeated Store calls.
func (m *Map[K, V]) StoreAll(entries ...Entry[K, V]) {
	m.checkCompare()
	switch len(entries) {
	case 0:
		return
	case 1:
		m.Store(entries[0].Key, entries[0].Value)
		return
	}
	batch := make([]mapEntry[K, V], len(entries))
	for i, e := range entries {
		batch[i] = mapEntry[K, V]{key: e.Key, value: e.Value}
	}
	m.storeBatch(batch)
}

// storeBatch stores batch's entries in m with Store semantics. It takes
// ownership of batch and may reorder and retain it.
func (m *Map[K, V]) storeBatch(batch []mapEntry[K, V]) {
	batch = m.normalize(batch)
	n, k := len(m.entries), len(batch)
	if k == 0 {
		return
	}
	// Fast paths: empty map, or every new key sorts after the existing ones.
	if n == 0 {
		m.entries = batch
		return
	}
	if m.compare(m.entries[n-1].key, batch[0].key) < 0 {
		m.entries = append(m.entries, batch...)
		return
	}

	// For small batches, binary-search each key and shift the existing entries
	// in place, avoiding a full-size allocation and per-entry comparisons.
	if k*bits.Len(uint(n)) < n {
		m.insertSorted(batch)
		return
	}

	merged := make([]mapEntry[K, V], 0, n+k)
	i, j := 0, 0
	for i < n && j < k {
		switch c := m.compare(m.entries[i].key, batch[j].key); {
		case c < 0:
			merged = append(merged, m.entries[i])
			i++
		case c > 0:
			merged = append(merged, batch[j])
			j++
		default:
			merged = append(merged, batch[j])
			i++
			j++
		}
	}
	merged = append(merged, m.entries[i:]...)
	merged = append(merged, batch[j:]...)
	clear(m.entries)
	m.entries = merged
}

// normalize sorts batch by key and removes duplicate keys, keeping the last
// entry of each run of equal keys.
func (m *Map[K, V]) normalize(batch []mapEntry[K, V]) []mapEntry[K, V] {
	strict := true
	for i := 1; i < len(batch); i++ {
		if m.compare(batch[i-1].key, batch[i].key) >= 0 {
			strict = false
			break
		}
	}
	if strict {
		return batch
	}
	// Sort with the original position as a tiebreaker: pdqsort is much faster
	// than a stable sort, and equal keys still end up in insertion order.
	type indexed struct {
		mapEntry[K, V]
		pos int
	}
	tmp := make([]indexed, len(batch))
	for i, e := range batch {
		tmp[i] = indexed{e, i}
	}
	slices.SortFunc(tmp, func(a, b indexed) int {
		if c := m.compare(a.key, b.key); c != 0 {
			return c
		}
		return cmp.Compare(a.pos, b.pos)
	})
	for i := range tmp {
		batch[i] = tmp[i].mapEntry
	}
	out := batch[:0]
	for i, e := range batch {
		if i+1 < len(batch) && m.compare(e.key, batch[i+1].key) == 0 {
			continue
		}
		out = append(out, e)
	}
	clear(batch[len(out):])
	return out
}

// insertSorted stores a sorted, duplicate-free, non-empty batch by locating
// each key with binary search, then shifting existing entries once, from the
// back, into their final positions.
func (m *Map[K, V]) insertSorted(batch []mapEntry[K, V]) {
	n := len(m.entries)
	type insertion struct {
		at    int
		entry mapEntry[K, V]
	}
	pending := make([]insertion, 0, len(batch))
	lo := 0
	for _, e := range batch {
		i := lo + sort.Search(n-lo, func(i int) bool {
			return m.compare(m.entries[lo+i].key, e.key) >= 0
		})
		if i < n && m.compare(m.entries[i].key, e.key) == 0 {
			m.entries[i].value = e.value
		} else {
			pending = append(pending, insertion{at: i, entry: e})
		}
		lo = i
	}
	if len(pending) == 0 {
		return
	}
	m.entries = slices.Grow(m.entries, len(pending))[:n+len(pending)]
	end := n
	for j := len(pending) - 1; j >= 0; j-- {
		p := pending[j]
		copy(m.entries[p.at+j+1:end+j+1], m.entries[p.at:end])
		m.entries[p.at+j] = p.entry
		end = p.at
	}
}

// LoadOrStore returns the existing value for key if present. Otherwise, it
// stores and returns value. The loaded result is true if the value was loaded.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	i, loaded := m.search(key)
	if loaded {
		return m.entries[i].value, true
	}
	m.entries = append(m.entries, mapEntry[K, V]{})
	copy(m.entries[i+1:], m.entries[i:])
	m.entries[i] = mapEntry[K, V]{key: key, value: value}
	return value, false
}

// LoadAndDelete removes the entry for key and returns its value, if any.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	if len(m.entries) == 0 {
		return value, false
	}
	i, loaded := m.search(key)
	if !loaded {
		return value, false
	}
	value = m.entries[i].value
	copy(m.entries[i:], m.entries[i+1:])
	last := len(m.entries) - 1
	var zero mapEntry[K, V]
	m.entries[last] = zero
	m.entries = m.entries[:last]
	return value, true
}

// Delete removes the entry for key. It does nothing if key is absent.
func (m *Map[K, V]) Delete(key K) {
	_, _ = m.LoadAndDelete(key)
}

// First returns the lowest key and its value. The ok result is false if m is
// empty.
func (m *Map[K, V]) First() (key K, value V, ok bool) {
	if len(m.entries) == 0 {
		return key, value, false
	}
	entry := m.entries[0]
	return entry.key, entry.value, true
}

// Last returns the highest key and its value. The ok result is false if m is
// empty.
func (m *Map[K, V]) Last() (key K, value V, ok bool) {
	if len(m.entries) == 0 {
		return key, value, false
	}
	entry := m.entries[len(m.entries)-1]
	return entry.key, entry.value, true
}

// LowerBound returns the first entry whose key does not sort before key. The ok
// result is false if no such entry exists.
func (m *Map[K, V]) LowerBound(key K) (foundKey K, value V, ok bool) {
	i, _ := m.search(key)
	if i == len(m.entries) {
		return foundKey, value, false
	}
	entry := m.entries[i]
	return entry.key, entry.value, true
}

// UpperBound returns the first entry whose key sorts after key. The ok result
// is false if no such entry exists.
func (m *Map[K, V]) UpperBound(key K) (foundKey K, value V, ok bool) {
	m.checkCompare()
	i := sort.Search(len(m.entries), func(i int) bool {
		return m.compare(m.entries[i].key, key) > 0
	})
	if i == len(m.entries) {
		return foundKey, value, false
	}
	entry := m.entries[i]
	return entry.key, entry.value, true
}

// Grow ensures space for another n entries without allocation.
func (m *Map[K, V]) Grow(n int) {
	m.entries = slices.Grow(m.entries, n)
}

// Clear removes all entries while retaining entry storage for reuse.
func (m *Map[K, V]) Clear() {
	clear(m.entries)
	m.entries = m.entries[:0]
}

// Shrink reallocates m's entry storage to exactly its current length. If m is
// empty, Shrink releases the storage entirely.
func (m *Map[K, V]) Shrink() {
	if len(m.entries) == cap(m.entries) {
		return
	}
	if len(m.entries) == 0 {
		m.entries = nil
		return
	}
	entries := make([]mapEntry[K, V], len(m.entries))
	copy(entries, m.entries)
	m.entries = entries
}

// Clone returns an independent copy of m.
func (m *Map[K, V]) Clone() *Map[K, V] {
	return &Map[K, V]{compare: m.compare, entries: slices.Clone(m.entries)}
}

// All returns an iterator over key and value pairs in ascending key order.
// Mutating m during iteration invalidates the iterator.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, entry := range m.entries {
			if !yield(entry.key, entry.value) {
				return
			}
		}
	}
}

// Backward returns an iterator over key and value pairs in descending key
// order. Mutating m during iteration invalidates the iterator.
func (m *Map[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, entry := range slices.Backward(m.entries) {
			if !yield(entry.key, entry.value) {
				return
			}
		}
	}
}

// Keys returns an iterator over keys in ascending order.
func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, entry := range m.entries {
			if !yield(entry.key) {
				return
			}
		}
	}
}

// Values returns an iterator over values in ascending key order.
func (m *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, entry := range m.entries {
			if !yield(entry.value) {
				return
			}
		}
	}
}

func (m *Map[K, V]) search(key K) (int, bool) {
	m.checkCompare()
	i := sort.Search(len(m.entries), func(i int) bool {
		return m.compare(m.entries[i].key, key) >= 0
	})
	return i, i < len(m.entries) && m.compare(m.entries[i].key, key) == 0
}

func (m *Map[K, V]) checkCompare() {
	if m.compare == nil {
		panic("container: Map must be initialized with NewMap or NewOrderedMap")
	}
}
