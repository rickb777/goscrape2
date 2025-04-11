package document

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/logger"
)

func TestFixURLReferences(t *testing.T) {
	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	u := mustParseURL("http://domain.com/content/")

	b := []byte(`<html lang="es"><head></head>
<body>
  <a href="https://domain.com/">Home</a>
  <a href="https://domain.com/wp-content/uploads/document.pdf" rel="doc">Guide</a>
  <img src="https://domain.com/content/test.jpg" srcset="https://domain.com/content/test-480w.jpg 480w, https://domain.com/content/test-800w.jpg 800w"/>
  <img src="/other.jpg" srcset="/other-480w.jpg 480w, /other-800w.jpg 800w"/>
</body></html>
`)

	doc, err := ParseHTML(u, u, bytes.NewReader(b))
	expect.Error(err).ToBeNil(t)

	ref, fixed, err := doc.FixURLReferences()
	expect.Error(err).ToBeNil(t)
	expect.Bool(fixed).ToBeTrue(t)

	expected := `<html lang="es"><head></head>
<body>
  <a href="../">Home</a>
  <a href="../wp-content/uploads/document.pdf" rel="doc">Guide</a>
  <img src="test.jpg" srcset="test-480w.jpg 480w, test-800w.jpg 800w"/>
  <img src="../other.jpg" srcset="../other-480w.jpg 480w, ../other-800w.jpg 800w"/>

</body></html>`
	expect.String(ref).ToEqual(t, expected)
}
