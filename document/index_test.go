package document

import (
	"bytes"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/rickb777/goscrape2/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	refs, err := doc.FindReferences()
	require.NoError(t, err)
	assert.Equal(t, 7, len(refs))
	assert.Contains(t, refs, mustParseURL("http://domain.com/wp-content/uploads/document.pdf"))
	assert.Contains(t, refs, mustParseURL("http://domain.com/some/things"))
	assert.Contains(t, refs, mustParseURL("http://domain.com/more/things/"))
	assert.Contains(t, refs, mustParseURL("http://domain.com/test.jpg"))
	assert.Contains(t, refs, mustParseURL("https://domain.com/test-480w.jpg"))
	assert.Contains(t, refs, mustParseURL("https://domain.com/test-800w.jpg"))
	assert.Contains(t, refs, mustParseURL("http://domain.com/js/func.min.js"))
}
