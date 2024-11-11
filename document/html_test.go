package document

import (
	"bytes"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixURLReferences(t *testing.T) {
	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		URL: "http://domain.com",
	}
	u, _ := url.Parse(cfg.URL)

	b := []byte(`
<html lang="es">
<a href="https://domain.com/wp-content/uploads/document.pdf" rel="doc">Guide</a>
<img src="https://domain.com/test.jpg" srcset="https://domain.com/test-480w.jpg 480w, https://domain.com/test-800w.jpg 800w"/> 
</html>
`)

	doc, err := ParseHTML(u, u, bytes.NewReader(b))
	require.NoError(t, err)

	ref, fixed, err := doc.FixURLReferences()
	require.NoError(t, err)
	assert.True(t, fixed)

	expected := "<html lang=\"es\"><head></head><body>" +
		"<a href=\"wp-content/uploads/document.pdf\" rel=\"doc\">Guide</a>\n" +
		"<img src=\"test.jpg\" srcset=\"test-480w.jpg 480w, test-800w.jpg 800w\"/> \n\n" +
		"</body></html>"
	assert.Equal(t, expected, string(ref))
}
