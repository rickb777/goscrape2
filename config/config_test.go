package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaders(t *testing.T) {
	headers := MakeHeaders([]struct {
		Key   string
		Value string
	}{
		{
			Key:   "a",
			Value: "b",
		},
		{
			Key:   "c",
			Value: "d",
		},
		{
			Key:   "c",
			Value: "e",
		},
	})
	assert.Equal(t, "b", headers.Get("a"))
	assert.Equal(t, []string{"d", "e"}, headers.Values("c"))
}
