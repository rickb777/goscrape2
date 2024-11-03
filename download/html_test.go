package download

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestFixURLReferences(t *testing.T) {
	logger.Logger = log.NewTestLogger(t)
	cfg := config.Config{
		URL: "http://domain.com",
	}
	u, _ := url.Parse(cfg.URL)
	d := Download{Config: cfg, StartURL: u}

	b := []byte(`
<html lang="es">
<a href="https://domain.com/wp-content/uploads/document.pdf" rel="doc">Guide</a>
<img src="https://domain.com/test.jpg" srcset="https://domain.com/test-480w.jpg 480w, https://domain.com/test-800w.jpg 800w"/> 
</html>
`)

	buf := &bytes.Buffer{}
	_, err := buf.Write(b)
	require.NoError(t, err)

	doc, err := html.Parse(buf)
	require.NoError(t, err)

	index := htmlindex.New()
	index.Index(d.StartURL, doc)

	ref, fixed, err := d.fixURLReferences(d.StartURL, doc, index)
	require.NoError(t, err)
	assert.True(t, fixed)

	expected := "<html lang=\"es\"><head></head><body>" +
		"<a href=\"wp-content/uploads/document.pdf\" rel=\"doc\">Guide</a>\n" +
		"<img src=\"test.jpg\" srcset=\"test-480w.jpg 480w, test-800w.jpg 800w\"/> \n\n" +
		"</body></html>"
	assert.Equal(t, expected, string(ref))
}
