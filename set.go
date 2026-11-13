package container

import (
	"cmp"
	"iter"
)

// Set is a non-concurrent collection of unique values that iterates in sorted
// order. Use NewSet or NewOrderedSet to initialize a Set before adding values.
type Set[E any] struct {
	values Map[E, struct{}]
}

// NewSet returns a Set ordered by compare, initialized with values.
// Duplicate values are stored once.
func NewSet[E any](compare Compare[E], values ...E) *Set[E] {
	s := &Set[E]{values: *NewMap[E, struct{}](compare)}
	s.AddAll(values...)
	return s
}

// NewOrderedSet returns a Set ordered by cmp.Compare, initialized with values.
// Duplicate values are stored once.
func NewOrderedSet[E cmp.Ordered](values ...E) *Set[E] {
	return NewSet[E](cmp.Compare[E], values...)
}

// Len returns the number of values in s.
func (s *Set[E]) Len() int { return s.values.Len() }

// Cap returns the capacity of s's value storage.
func (s *Set[E]) Cap() int { return s.values.Cap() }

// Contains reports whether value is in s.
func (s *Set[E]) Contains(value E) bool {
	_, ok := s.values.Load(value)
	return ok
}

// Add inserts value. It reports whether value was newly added.
func (s *Set[E]) Add(value E) bool {
	_, loaded := s.values.LoadOrStore(value, struct{}{})
	return !loaded
}

// AddAll inserts every value in values and returns the number of values that
// were newly added. Duplicates, within values or already in s, count once.
func (s *Set[E]) AddAll(values ...E) (added int) {
	before := s.values.Len()
	switch len(values) {
	case 0:
		s.values.checkCompare()
		return 0
	case 1:
		if s.Add(values[0]) {
			return 1
		}
		return 0
	}
	batch := make([]mapEntry[E, struct{}], len(values))
	for i, v := range values {
		batch[i].key = v
	}
	s.values.storeBatch(batch)
	return s.values.Len() - before
}

// Delete removes value. It reports whether value was present.
func (s *Set[E]) Delete(value E) bool {
	_, loaded := s.values.LoadAndDelete(value)
	return loaded
}

// First returns the lowest value. The ok result is false if s is empty.
func (s *Set[E]) First() (value E, ok bool) {
	value, _, ok = s.values.First()
	return value, ok
}

// Last returns the highest value. The ok result is false if s is empty.
func (s *Set[E]) Last() (value E, ok bool) {
	value, _, ok = s.values.Last()
	return value, ok
}

// LowerBound returns the first stored value that does not sort before value.
func (s *Set[E]) LowerBound(value E) (found E, ok bool) {
	found, _, ok = s.values.LowerBound(value)
	return found, ok
}

// UpperBound returns the first stored value that sorts after value.
func (s *Set[E]) UpperBound(value E) (found E, ok bool) {
	found, _, ok = s.values.UpperBound(value)
	return found, ok
}

// Grow ensures space for another n values without allocation.
func (s *Set[E]) Grow(n int) { s.values.Grow(n) }

// Clear removes all values while retaining storage for reuse.
func (s *Set[E]) Clear() { s.values.Clear() }

// Shrink reallocates s's value storage to exactly its current length.
func (s *Set[E]) Shrink() { s.values.Shrink() }

// Clone returns an independent copy of s.
func (s *Set[E]) Clone() *Set[E] {
	return &Set[E]{values: *s.values.Clone()}
}

// All returns an iterator over values in ascending order. Mutating s during
// iteration invalidates the iterator.
func (s *Set[E]) All() iter.Seq[E] { return s.values.Keys() }

// Backward returns an iterator over values in descending order. Mutating s
// during iteration invalidates the iterator.
func (s *Set[E]) Backward() iter.Seq[E] {
	return func(yield func(E) bool) {
		for value := range s.values.Backward() {
			if !yield(value) {
				return
			}
		}
	}
}
