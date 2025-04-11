package htmlindex

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/rickb777/expect"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestIndex(t *testing.T) {
	input := []byte(`
<html lang="es">
<script src="//api.html5media.info/1.1.8/html5media.min.js"></script>
<a href="https://domain.com/wp-content/uploads/document%2Bindex.pdf" rel="doc">Guide</a>
<a href="https://domain.com/about.html">About</a>
<a href="things">Things</a>
<img src="/test%24file.jpg"/> 
<script src="/func.min.js"></script> 
</html>
`)

	idx := New()

	doc, err := html.Parse(bytes.NewReader(input))
	expect.Error(err).ToBeNil(t)

	idx.Index(mustParse("https://domain.com/"), doc)

	// check <a> tag
	{
		references, err := idx.URLs(atom.A)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 3)

		expect.String(references[0].String()).ToBe(t, "https://domain.com/about.html")
		expect.String(references[0].Path).ToBe(t, "/about.html")

		expect.String(references[1].String()).ToBe(t, "https://domain.com/things")
		expect.String(references[1].Path).ToBe(t, "/things")

		expect.String(references[2].String()).ToBe(t, "https://domain.com/wp-content/uploads/document%2Bindex.pdf")
		expect.String(references[2].Path).ToBe(t, "/wp-content/uploads/document+index.pdf")

		urls := idx.Nodes(atom.A)
		expect.Map(urls).ToHaveLength(t, 3)
		nodes, ok := urls[("https://domain.com/wp-content/uploads/document%2Bindex.pdf")]
		expect.Bool(ok).ToBeTrue(t)
		expect.Slice(nodes).ToHaveLength(t, 1)
		expect.String(nodes[0].Data).ToBe(t, "a")
	}
	// check <img> tag
	{
		references, err := idx.URLs(atom.Img)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 1)
		expect.String(references[0].String()).ToBe(t, "https://domain.com/test%24file.jpg")
		expect.String(references[0].Path).ToBe(t, "/test$file.jpg")
	}
	// check <script> tag
	{
		references, err := idx.URLs(atom.Script)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 2)
		expect.String(references[0].String()).ToBe(t, "https://api.html5media.info/1.1.8/html5media.min.js")
		expect.String(references[1].String()).ToBe(t, "https://domain.com/func.min.js")
	}
	// check for non-existent tag
	{
		references, err := idx.URLs(0)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToBeEmpty(t)
		urls := idx.Nodes(0)
		expect.Map(urls).ToBeEmpty(t)
	}
}

func TestIndexWithBase(t *testing.T) {
	input := []byte(`
<html lang="es"><head><base href=' https://domain.com '/></head>
<body>
<a href=" /about.html ">About</a>
<a href="things">Things</a>
</body>
</html>
`)

	idx := New()

	doc, err := html.Parse(bytes.NewReader(input))
	expect.Error(err).ToBeNil(t)

	idx.Index(mustParse("https://www.domain.com/"), doc)

	// check <a> tag
	{
		references, err := idx.URLs(atom.A)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 2)

		expect.String(references[0].String()).ToBe(t, "https://domain.com/about.html")
		expect.String(references[0].Path).ToBe(t, "/about.html")

		expect.String(references[1].String()).ToBe(t, "https://domain.com/things")
		expect.String(references[1].Path).ToBe(t, "/things")

		urls := idx.Nodes(atom.A)
		expect.Map(urls).ToHaveLength(t, 2)
		nodes, ok := urls[("https://domain.com/about.html")]
		expect.Bool(ok).ToBeTrue(t)
		expect.Slice(nodes).ToHaveLength(t, 1)
		expect.String(nodes[0].Data).ToBe(t, "a")
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

	idx := New()

	doc, err := html.Parse(bytes.NewReader(input))
	expect.Error(err).ToBeNil(t)

	idx.Index(mustParse("https://domain.com/"), doc)

	{
		references, err := idx.URLs(atom.Img)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 3)
		expect.String(references[0].String()).ToBe(t, "https://domain.com/test-480w.jpg")
		expect.String(references[1].String()).ToBe(t, "https://domain.com/test-800w.jpg")
		expect.String(references[2].String()).ToBe(t, "https://domain.com/test.jpg")
	}
	{
		references, err := idx.URLs(atom.Body)
		expect.Error(err).ToBeNil(t)
		expect.Slice(references).ToHaveLength(t, 1)
		expect.String(references[0].String()).ToBe(t, "https://domain.com/bg.jpg")
	}
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
