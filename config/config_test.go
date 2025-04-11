package config

import (
	"github.com/rickb777/expect"
	"testing"
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
	expect.String(headers.Get("a")).ToBe(t, "b")
	expect.Slice(headers.Values("c")).ToBe(t, "d", "e")
}
