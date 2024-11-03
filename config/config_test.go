package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaders(t *testing.T) {
	headers := MakeHeaders([]string{"a:b", "c:d:e"})
	assert.Equal(t, "b", headers.Get("a"))
	assert.Equal(t, "d:e", headers.Get("c"))
}
