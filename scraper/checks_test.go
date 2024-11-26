package scraper

import (
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/url"
	"testing"
)

func MustParseURL(s string) *url.URL {
	u, e := url.Parse(s)
	if e != nil {
		panic(e)
	}
	return u
}

func TestShouldURLBeDownloaded(t *testing.T) {
	setup()
	startURL := "https://example.org/#fragment"

	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", "")
	stub.GivenResponse(http.StatusOK, "https://example.org/page2", "text/html", "")
	stub.GivenResponse(http.StatusOK, "https://example.org/sub/", "text/html", "")
	stub.GivenResponse(http.StatusOK, "https://example.org/style.css", "text/css", "")

	scraper := newTestScraper(t, startURL, stub)
	require.NotNil(t, scraper)

	scraper.processed.Add("/ok/done")
	scraper.includes, _ = filter.New([]string{"/ok"})
	scraper.excludes, _ = filter.New([]string{"/../bad"})

	cases := []struct {
		item     *url.URL
		depth    int
		expected bool
	}{
		{item: MustParseURL("http://example.org/ok/wanted"), expected: true},
		{item: MustParseURL("http://example.org/ok/toodeep"), depth: 11, expected: false},
		{item: MustParseURL("http://example.org/oktoodeep"), depth: 12, expected: false},
		{item: MustParseURL("ftp://example.org/ok"), expected: false},
		{item: MustParseURL("https://example.org/ok/done"), expected: false},
		{item: MustParseURL("https://other.org/ok"), expected: false},
		{item: MustParseURL("https://example.org/ok/bad"), expected: false},
	}

	for _, c := range cases {
		result := scraper.shouldURLBeDownloaded(c.item, c.depth)
		assert.Equal(t, c.expected, result, c.item.String())
	}
}
