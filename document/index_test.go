package document

import (
	"bytes"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/logger"
)

func mustParseURL(s string) *url.URL {
	u, e := url.Parse(s)
	if e != nil {
		panic(e)
	}
	return u
}

func TestFindReferences(t *testing.T) {
	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	u := mustParseURL("http://domain.com")

	b := []byte(`<html lang="es"><head></head>
<body>
  <a href="/wp-content/uploads/document.pdf" rel="doc">Guide</a>
  <a href="/some/things">Some things</a>
  <a href="/more/things/">More things</a>
  <img src="/test.jpg" srcset="https://domain.com/test-480w.jpg 480w, https://domain.com/test-800w.jpg 800w"/>
  <script src="/js/func.min.js"></script> 
</body></html>
`)

	doc, err := ParseHTML(u, u, bytes.NewReader(b))
	expect.Error(err).ToBeNil(t)

	refs, err := doc.FindReferences()
	expect.Error(err).ToBeNil(t)
	expect.Slice(refs).ToHaveLength(t, 7)
	expect.Slice(refs).ToContainAll(t,
		mustParseURL("http://domain.com/wp-content/uploads/document.pdf"),
		mustParseURL("http://domain.com/some/things"),
		mustParseURL("http://domain.com/more/things/"),
		mustParseURL("http://domain.com/test.jpg"),
		mustParseURL("https://domain.com/test-480w.jpg"),
		mustParseURL("https://domain.com/test-800w.jpg"),
		mustParseURL("http://domain.com/js/func.min.js"))
}
