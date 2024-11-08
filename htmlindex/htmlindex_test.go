package htmlindex

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestIndex(t *testing.T) {
	input := []byte(`
<html lang="es">
<script src="//api.html5media.info/1.1.8/html5media.min.js"></script>
<a href="https://domain.com/wp-content/uploads/document%2Bindex.pdf" rel="doc">Guide</a>
<a href="https://domain.com/about.html">About</a>
<img src="/test%24file.jpg"/> 
<script src="/func.min.js"></script> 
</html>
`)

	idx := testSetup(t, input)

	// check a tag
	{
		references, err := idx.URLs("a")
		require.NoError(t, err)
		require.Len(t, references, 2)

		u2 := "https://domain.com/about.html"
		assert.Equal(t, u2, references[0].String())
		assert.Equal(t, "/about.html", references[0].Path)

		u1 := "https://domain.com/wp-content/uploads/document%2Bindex.pdf"
		assert.Equal(t, u1, references[1].String())
		assert.Equal(t, "/wp-content/uploads/document+index.pdf", references[1].Path)

		urls := idx.Nodes("a")
		require.Len(t, urls, 2)
		nodes, ok := urls[u1]
		require.True(t, ok)
		require.Len(t, nodes, 1)
		assert.Equal(t, "a", nodes[0].Data)
	}
	// check img tag
	{
		references, err := idx.URLs("img")
		require.NoError(t, err)
		require.Len(t, references, 1)

		tagURL := "https://domain.com/test%24file.jpg"
		assert.Equal(t, tagURL, references[0].String())
		assert.Equal(t, "/test$file.jpg", references[0].Path)
	}
	// check script tag
	{
		references, err := idx.URLs("script")
		require.NoError(t, err)
		require.Len(t, references, 2)

		tagURL1 := "https://api.html5media.info/1.1.8/html5media.min.js"
		assert.Equal(t, tagURL1, references[0].String())

		tagURL2 := "https://domain.com/func.min.js"
		assert.Equal(t, tagURL2, references[1].String())
	}
	// check for non-existent tag
	{
		references, err := idx.URLs("not-existing")
		require.NoError(t, err)
		require.Empty(t, references)
		urls := idx.Nodes("not-existing")
		require.Empty(t, urls)
	}
}

func TestIndexImg(t *testing.T) {
	input := []byte(`
<html lang="es">
<body background="bg.jpg"></body>
<img src="test.jpg" srcset="test-480w.jpg 480w, test-800w.jpg 800w"/> 
</body>
</html>
`)

	idx := testSetup(t, input)
	{
		references, err := idx.URLs(ImgTag)
		require.NoError(t, err)
		require.Len(t, references, 3)
		assert.Equal(t, "https://domain.com/test-480w.jpg", references[0].String())
		assert.Equal(t, "https://domain.com/test-800w.jpg", references[1].String())
		assert.Equal(t, "https://domain.com/test.jpg", references[2].String())
	}
	{
		references, err := idx.URLs(BodyTag)
		require.NoError(t, err)
		require.Len(t, references, 1)
		assert.Equal(t, "https://domain.com/bg.jpg", references[0].String())
	}
}

func testSetup(t *testing.T, input []byte) *Index {
	t.Helper()

	buf := &bytes.Buffer{}
	_, err := buf.Write(input)
	require.NoError(t, err)

	doc, err := html.Parse(buf)
	require.NoError(t, err)

	ur, err := url.Parse("https://domain.com/")
	require.NoError(t, err)

	idx := New()
	idx.Index(ur, doc)

	return idx
}
