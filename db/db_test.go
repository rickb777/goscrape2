package db

import (
	"github.com/rickb777/acceptable/header"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"testing"
)

func TestDB(t *testing.T) {
	store := openDB(".")
	defer os.Remove("./" + dbName)
	defer store.Close()

	u1 := mustParse("http://example.org/")
	store.Store(u1, header.ETag{Hash: "h1a"}, header.ETag{Hash: "h1b"})

	u2 := mustParse("http://example.org/a/b/c/")
	store.Store(u2, header.ETag{
		Hash: "h2",
		Weak: false,
	})

	u3 := mustParse("http://example.org/a/b/c/style.css")
	store.Store(u3, header.ETag{
		Hash: "h3",
		Weak: true,
	})

	v1 := store.Lookup(u1)
	assert.Equal(t, v1, header.ETags{{Hash: "h1a"}, {Hash: "h1b"}})

	v2 := store.Lookup(u2)
	assert.Equal(t, v2, header.ETags{{Hash: "h2", Weak: false}})

	v3 := store.Lookup(u3)
	assert.Equal(t, v3, header.ETags{{Hash: "h3", Weak: true}})
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
