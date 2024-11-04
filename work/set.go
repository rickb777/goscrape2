package work

import "sync"

// Empty holds nothing; it is used for set values.
type Empty struct{}

// Set contains distinct values of comparable type T. It can be accessed
// and altered concurrently.
//
// Note that literals need empty values, so will be of the form
//
//	Set[int]{2: {}, 4: {}}
type Set[T comparable] struct {
	m  map[T]Empty
	mu sync.Mutex
}

// NewSet creates a new set based on some optional initial values.
func NewSet[T comparable](slice ...T) *Set[T] {
	m := make(map[T]Empty)

	for _, val := range slice {
		m[val] = Empty{}
	}

	return &Set[T]{m: m}
}

func (s *Set[T]) Slice() []T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.m) == 0 {
		return nil
	}

	r := make([]T, 0, len(s.m))
	for k := range s.m {
		r = append(r, k)
	}
	return r
}

func (s *Set[T]) Add(val ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range val {
		s.m[v] = Empty{}
	}
}

// AddIfAbsent adds val to the set if absent, returning true.
// Otherwise, it returns false.
func (s *Set[T]) AddIfAbsent(val T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.m[val]
	if exists {
		return false
	}

	s.m[val] = Empty{}
	return true
}

func (s *Set[T]) Contains(val T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.m[val]
	return exists
}

func (s *Set[T]) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.m)
}
