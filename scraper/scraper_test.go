package scraper

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestScraper(t *testing.T, startURL string, stub *stubclient.Client) *Scraper {
	setup()
	t.Helper()

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{MaxDepth: 10}
	scraper, err := New(cfg, MustParseURL(startURL), afero.NewMemMapFs())
	require.NoError(t, err)
	require.NotNil(t, scraper)

	scraper.client = stub

	return scraper
}

func TestScraperLinks(t *testing.T) {
	indexPage := `
<html>
<head>
<link href=' //example.org/style.css#fragment' rel='stylesheet' type='text/css'>
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

	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", indexPage)
	stub.GivenResponse(http.StatusOK, "https://example.org/page2", "text/html", page2)
	stub.GivenResponse(http.StatusOK, "https://example.org/sub/", "text/html", indexPage)
	stub.GivenResponse(http.StatusOK, "https://example.org/style.css", "text/css", "")

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

	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", indexPage)
	stub.GivenResponse(http.StatusOK, "https://example.org/bg.gif", "image/gif", "")

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
