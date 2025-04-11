package scraper

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/stubclient"
	"github.com/spf13/afero"
)

func newTestScraper(t *testing.T, startURL string, stub *stubclient.Client) *Scraper {
	setup()
	t.Helper()

	cfg := config.Config{MaxDepth: 10}
	sc, err := New(cfg, mustParseURL(startURL), afero.NewMemMapFs())
	expect.Error(err).ToBeNil(t)
	expect.Any(sc).Not().ToBeNil(t)

	sc.Client = stub
	return sc
}

func TestScraperLinks(t *testing.T) {
	indexPage := `
<html>
<head>
<link href=' //example.org/style.css#fragment' rel='stylesheet' type='text/css'>
</head>
<body>
<a href="/page2">Example 2</a>
<a href="/page3/">Example 3</a>
<a href="/page4">Example 4</a>
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

	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", indexPage)
	stub.GivenResponse(http.StatusOK, "https://example.org/page2", "text/html", page2)
	stub.GivenResponse(http.StatusOK, "https://example.org/page3/", "text/html", page2) // same
	stub.GivenResponse(http.StatusNotFound, "https://example.org/page4", "*/*", "")
	//stub.GivenResponse(http.StatusNotFound, "https://example.org/page4/", "*/*", "")
	stub.GivenResponse(http.StatusOK, "https://example.org/sub/", "text/html", indexPage)
	stub.GivenResponse(http.StatusOK, "https://example.org/style.css", "text/css", "")

	scraper := newTestScraper(t, "https://example.org/#fragment", stub)
	expect.Any(scraper).Not().ToBeNil(t)

	ctx := context.Background()
	err := scraper.Start(ctx)
	expect.Error(err).ToBeNil(t)

	expectedProcessed := []string{
		"/",
		"/page2",
		"/page3/",
		"/page4",
		"/style.css",
		"/sub/",
	}
	actualProcessed := scraper.processed.Slice()
	slices.Sort(actualProcessed)
	expect.Slice(actualProcessed).ToBe(t, expectedProcessed...)
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
	expect.Any(scraper).Not().ToBeNil(t)

	ctx := context.Background()
	err := scraper.Start(ctx)
	expect.Error(err).ToBeNil(t)

	expectedProcessed := []string{
		"/",
		"/bg.gif",
	}
	actualProcessed := scraper.processed.Slice()
	slices.Sort(actualProcessed)
	expect.Slice(actualProcessed).ToBe(t, expectedProcessed...)
}
