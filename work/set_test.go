package work

import (
	"github.com/rickb777/expect"
	"slices"
	"testing"
)

func TestNewSet(t *testing.T) {
	stringTests := []struct {
		input          []string
		expectedOutput []string
	}{
		{input: []string{}, expectedOutput: nil},
		{input: []string{"hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob", "hi", "bob"}, expectedOutput: []string{"bob", "hi"}},
	}

	for i, test := range stringTests {
		output := NewSet(test.input...).Slice()
		slices.Sort(output) // otherwise the test would be unstable
		expect.Slice(output).I(i).ToBe(t, test.expectedOutput...)
	}
}

func TestSetAdd(t *testing.T) {
	s := NewSet[int]()
	s.Add(1, 3, 5)
	expect.Number(s.Size()).ToBe(t, 3)
	expect.Bool(s.Contains(3)).ToBeTrue(t)
	expect.Bool(s.Contains(5)).ToBeTrue(t)

	sl := s.Slice()
	slices.Sort(sl)
	expect.Slice(sl).ToBe(t, 1, 3, 5)
}
