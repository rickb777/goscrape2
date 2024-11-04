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
		expectedOutput []string
	}{
		{input: []string{}, expectedOutput: nil},
		{input: []string{"hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob"}, expectedOutput: []string{"bob", "hi"}},
	}

	for _, test := range stringTests {
		output := NewSet(test.input...).Slice()
		slices.Sort(output) // otherwise the test would be unstable
		assert.Equal(test.expectedOutput, output)
	}
}

func TestSetAdd(t *testing.T) {
	assert := assertpkg.New(t)
	s := NewSet[int]()
	s.Add(1, 3, 5)
	assert.Equal(3, s.Size())
	assert.True(s.Contains(3))
	assert.True(s.Contains(5))

	sl := s.Slice()
	slices.Sort(sl)
	assert.Equal([]int{1, 3, 5}, sl)
}
