package work

// Empty holds nothing; it is used for set values.
type Empty struct{}

// Set contains distinct values of comparable type V.
// It is essentially a `map`, so all built-in map operations will work.
// Note that literals need empty values, so will be of the form
//
//	Set[int]{2: {}, 4: {}}
type Set[V comparable] map[V]Empty

func (s Set[V]) Slice() []V {
	r := make([]V, 0, len(s))
	for k := range s {
		r = append(r, k)
	}
	return r
}

func (s Set[V]) Add(val ...V) {
	for _, v := range val {
		s[v] = Empty{}
	}
}

func (s Set[V]) Contains(val V) bool {
	_, exists := s[val]
	return exists
}

// NewSet creates a new set based on some optional initial values.
func NewSet[S comparable](slice ...S) Set[S] {
	m := make(map[S]Empty)

	for _, val := range slice {
		m[val] = Empty{}
	}

	return m
}
