package db

import (
	"github.com/rickb777/acceptable/header"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStringRepresentation(t *testing.T) {
	buf := &strings.Builder{}
	t1 := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	textHtml := header.ContentType{Type: "text", Subtype: "html"}

	writeItem(buf, "k1", Item{File: "./foo.html", Expires: t1})
	writeItem(buf, "k2", Item{File: "./foo.html", Content: textHtml, Expires: t1.Add(time.Hour), ETags: `"abc123"`})
	writeItem(buf, "k3", Item{File: "./foo.html", ETags: `"def123"`})
	writeItem(buf, "k4", Item{Location: "/foo/bar.html"})

	s := strings.Split(buf.String(), "\n")

	assert.Equal(t, `k1	-	./foo.html	-	2000-01-01T01:01:01Z	-`, s[0])
	assert.Equal(t, `k2	-	./foo.html	text/html	2000-01-01T02:01:01Z	"abc123"`, s[1])
	assert.Equal(t, `k3	-	./foo.html	-	-	"def123"`, s[2])
	assert.Equal(t, `k4	/foo/bar.html	-	-	-	-`, s[3])
}

func TestDB(t *testing.T) {
	fs := afero.NewOsFs()
	store1 := OpenDB(".", fs)
	defer os.Remove("./" + FileName)
	defer store1.Close()

	t1 := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

	u1 := mustParse("http://example.org/")
	store1.Store(u1, Item{ETags: `"h1a", "h1b"`})

	u2 := mustParse("http://example.org/a/b/c/index.html#sec1")
	store1.Store(u2, Item{ETags: `"h2"`, Expires: t1})

	u3 := mustParse("http://example.org/a/b/c/style.css")
	store1.Store(u3, Item{ETags: `W/"h3"`})

	v1 := store1.Lookup(u1)
	assert.Equal(t, Item{ETags: `"h1a", "h1b"`}, v1)

	v2 := store1.Lookup(u2)
	assert.Equal(t, `"h2"`, v2.ETags)
	assert.True(t, v2.Expires.Equal(t1), "%s %s", t1, v2.Expires)

	v3 := store1.Lookup(u3)
	assert.Equal(t, Item{ETags: `W/"h3"`}, v3)

	store1.Close()
	store1 = nil

	//-------------------------------------------

	store2 := OpenDB(".", fs)
	store2.Store(u3, Item{})

	w1 := store2.Lookup(u1)
	assert.Equal(t, Item{ETags: `"h1a", "h1b"`}, w1)

	w2 := store2.Lookup(u2)
	assert.Equal(t, `"h2"`, w2.ETags)
	assert.True(t, w2.Expires.Equal(t1), "%s %s", t1, w2.Expires)

	w3 := store2.Lookup(u3)
	assert.Equal(t, "", w3.ETags)
	assert.True(t, w3.Expires.IsZero())
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
