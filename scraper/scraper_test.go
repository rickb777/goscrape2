package scraper

import (
	"bytes"
	"context"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/logger"
	"github.com/rickb777/acceptable/headername"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubClient struct {
	responses map[string]*http.Response // more configurable responses
}

func (c *stubClient) response(url, contentType, body string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	rdr := bytes.NewReader([]byte(body))
	resp := &http.Response{
		Request:    req,
		Header:     http.Header{headername.ContentType: []string{contentType}},
		Body:       io.NopCloser(rdr),
		StatusCode: http.StatusOK,
	}
	if c.responses == nil {
		c.responses = make(map[string]*http.Response)
	}
	c.responses[url] = resp
}

func (c *stubClient) Do(req *http.Request) (resp *http.Response, err error) {
	ur := req.URL.String()
	resp, ok := c.responses[ur]
	if ok {
		return resp, nil
	}
	panic(fmt.Sprintf("url '%s' not found in test data", ur))
}

//-------------------------------------------------------------------------------------------------

func newTestScraper(t *testing.T, startURL string, stub *stubClient) *Scraper {
	t.Helper()

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		URL:      startURL,
		MaxDepth: 10,
	}
	scraper, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, scraper)

	scraper.fs = afero.NewMemMapFs()

	scraper.client = stub

	return scraper
}

func TestScraperLinks(t *testing.T) {
	indexPage := `
<html>
<head>
<link href=' https://example.org/style.css#fragment' rel='stylesheet' type='text/css'>
</head>
<body>
<a href="https://example.org/page2">Example</a>
</body>
</html>
`

	page2 := `
<html>
<body>

<!--link to index with fragment-->
<a href="/#fragment">a</a>
<!--link to page with fragment-->
<a href="/sub/#fragment">a</a>

</body>
</html>
`

	startURL := "https://example.org/#fragment" // start page with fragment

	stub := &stubClient{}
	stub.response("https://example.org/", "text/html", indexPage)
	stub.response("https://example.org/page2", "text/html", page2)
	stub.response("https://example.org/sub/", "text/html", indexPage)
	stub.response("https://example.org/style.css", "text/css", "")

	scraper := newTestScraper(t, startURL, stub)
	require.NotNil(t, scraper)

	ctx := context.Background()
	err := scraper.Start(ctx)
	require.NoError(t, err)

	expectedProcessed := []string{
		"/",
		"/page2",
		"/style.css",
		"/sub/",
	}
	actualProcessed := scraper.processed.Slice()
	slices.Sort(actualProcessed)
	assert.Equal(t, expectedProcessed, actualProcessed)
}

func TestScraperAttributes(t *testing.T) {
	indexPage := `
<html>
<head>
</head>

<body background="bg.gif">

<!--embedded image-->
<img src='data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs%3D=' />

</body>
</html>
`

	startURL := "https://example.org/"

	stub := &stubClient{}
	stub.response("https://example.org/", "text/html", indexPage)
	stub.response("https://example.org/bg.gif", "image/gif", "")

	scraper := newTestScraper(t, startURL, stub)
	require.NotNil(t, scraper)

	ctx := context.Background()
	err := scraper.Start(ctx)
	require.NoError(t, err)

	expectedProcessed := []string{
		"/",
		"/bg.gif",
	}
	actualProcessed := scraper.processed.Slice()
	slices.Sort(actualProcessed)
	assert.Equal(t, expectedProcessed, actualProcessed)
}
