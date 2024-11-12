package db

import (
	"github.com/rickb777/acceptable/header"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"testing"
)

func TestDB(t *testing.T) {
	fs := afero.NewOsFs()
	store1 := OpenDB(".", fs)
	defer os.Remove("./" + FileName)
	defer store1.Close()

	u1 := mustParse("http://example.org/")
	store1.Store(u1, header.ETags{{Hash: "h1a"}, {Hash: "h1b"}})

	u2 := mustParse("http://example.org/a/b/c/index.html#sec1")
	store1.Store(u2, header.ETags{{
		Hash: "h2",
		Weak: false,
	}})

	u3 := mustParse("http://example.org/a/b/c/style.css")
	store1.Store(u3, header.ETags{{
		Hash: "h3",
		Weak: true,
	}})

	v1 := store1.Lookup(u1)
	assert.Equal(t, v1, header.ETags{{Hash: "h1a"}, {Hash: "h1b"}})

	v2 := store1.Lookup(u2)
	assert.Equal(t, v2, header.ETags{{Hash: "h2", Weak: false}})

	v3 := store1.Lookup(u3)
	assert.Equal(t, v3, header.ETags{{Hash: "h3", Weak: true}})

	store1.Close()
	store1 = nil

	//-------------------------------------------

	store2 := OpenDB(".", fs)
	store2.Store(u3, nil)

	w1 := store2.Lookup(u1)
	assert.Equal(t, w1, header.ETags{{Hash: "h1a"}, {Hash: "h1b"}})

	w2 := store2.Lookup(u2)
	assert.Equal(t, w2, header.ETags{{Hash: "h2", Weak: false}})

	w3 := store2.Lookup(u3)
	assert.Nil(t, w3)
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
