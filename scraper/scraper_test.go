package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/cornelk/gotokit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestScraper(t *testing.T, startURL string, urls map[string][]byte) *Scraper {
	t.Helper()

	logger.Logger = log.NewTestLogger(t)
	cfg := config.Config{
		URL: startURL,
	}
	scraper, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, scraper)

	download.CreateDirectory = func(_ string) error {
		return nil
	}
	download.WriteFile = func(_ *url.URL, _ string, _ io.Reader) error {
		return nil
	}
	download.FileExists = func(_ string) bool {
		return false
	}
	download.DownloadURL = func(_ context.Context, _ *download.Download, u *url.URL) (*http.Response, error) {
		ur := u.String()
		b, ok := urls[ur]
		if !ok {
			return nil, fmt.Errorf("url '%s' not found in test data", ur)
		}
		req, _ := http.NewRequest(http.MethodGet, ur, nil)
		body := bytes.NewReader(b)
		return &http.Response{
			Request: req,
			Header:  http.Header{"Content-Type": []string{"text/html"}},
			Body:    io.NopCloser(body),
		}, nil
	}

	return scraper
}

func TestScraperLinks(t *testing.T) {
	indexPage := []byte(`
<html>
<head>
<link href=' https://example.org/style.css#fragment' rel='stylesheet' type='text/css'>
</head>
<body>
<a href="https://example.org/page2">Example</a>
</body>
</html>
`)

	page2 := []byte(`
<html>
<body>

<!--link to index with fragment-->
<a href="/#fragment">a</a>
<!--link to page with fragment-->
<a href="/sub/#fragment">a</a>

</body>
</html>
`)

	css := []byte(``)

	startURL := "https://example.org/#fragment" // start page with fragment
	urls := map[string][]byte{
		"https://example.org/":          indexPage,
		"https://example.org/page2":     page2,
		"https://example.org/sub/":      indexPage,
		"https://example.org/style.css": css,
	}

	scraper := newTestScraper(t, startURL, urls)
	require.NotNil(t, scraper)

	ctx := context.Background()
	err := scraper.Start(ctx)
	require.NoError(t, err)

	expectedProcessed := work.Set[string]{
		"/":          {},
		"/page2":     {},
		"/sub/":      {},
		"/style.css": {},
	}
	assert.Equal(t, expectedProcessed, scraper.processed)
}

func TestScraperAttributes(t *testing.T) {
	indexPage := []byte(`
<html>
<head>
</head>

<body background="bg.gif">

<!--embedded image-->
<img src='data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs%3D=' />

</body>
</html>
`)
	empty := []byte(``)

	startURL := "https://example.org/"
	urls := map[string][]byte{
		"https://example.org/":       indexPage,
		"https://example.org/bg.gif": empty,
	}

	scraper := newTestScraper(t, startURL, urls)
	require.NotNil(t, scraper)

	ctx := context.Background()
	err := scraper.Start(ctx)
	require.NoError(t, err)

	expectedProcessed := work.Set[string]{
		"/":       {},
		"/bg.gif": {},
	}
	assert.Equal(t, expectedProcessed, scraper.processed)
}
