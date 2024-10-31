package work

import (
	assertpkg "github.com/stretchr/testify/assert"
	"slices"
	"testing"
)

func TestNewSet(t *testing.T) {
	assert := assertpkg.New(t)
	stringTests := []struct {
		input          []string
		expectedOutput Set[string]
	}{
		{input: []string{}, expectedOutput: Set[string]{}},
		{input: []string{"hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob"}, expectedOutput: Set[string]{"hi": {}, "bob": {}}},
	}

	for _, test := range stringTests {
		output := NewSet(test.input...)
		assert.Equal(test.expectedOutput, output)
	}
}

func TestSetAdd(t *testing.T) {
	assert := assertpkg.New(t)
	s := Set[int]{}
	s.Add(1, 3, 5)
	assert.Equal(3, len(s))
	assert.True(s.Contains(3))
	assert.True(s.Contains(5))

	sl := s.Slice()
	slices.Sort(sl)
	assert.Equal([]int{1, 3, 5}, sl)
}
