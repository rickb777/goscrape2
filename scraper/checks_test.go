package scraper

import (
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/work"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	empty := []byte("")

	startURL := "https://example.org/#fragment"
	urls := map[string][]byte{
		"https://example.org/":          empty,
		"https://example.org/page2":     empty,
		"https://example.org/sub/":      empty,
		"https://example.org/style.css": empty,
	}

	scraper := newTestScraper(t, startURL, urls)
	require.NotNil(t, scraper)

	scraper.processed.Add("/ok/done")
	scraper.includes, _ = filter.New([]string{"/ok"})
	scraper.excludes, _ = filter.New([]string{"/../bad"})

	cases := []struct {
		item     work.Item
		expected bool
	}{
		{item: work.Item{URL: MustParseURL("http://example.org/ok/wanted")}, expected: true},
		{item: work.Item{URL: MustParseURL("http://example.org/ok/toodeep"), Depth: 10}, expected: false},
		{item: work.Item{URL: MustParseURL("http://example.org/oktoodeep"), Depth: 11}, expected: false},
		{item: work.Item{URL: MustParseURL("ftp://example.org/ok")}, expected: false},
		{item: work.Item{URL: MustParseURL("https://example.org/ok/done")}, expected: false},
		{item: work.Item{URL: MustParseURL("https://other.org/ok")}, expected: false},
		{item: work.Item{URL: MustParseURL("https://example.org/ok/bad")}, expected: false},
	}

	for _, c := range cases {
		result := scraper.shouldURLBeDownloaded(c.item)
		assert.Equal(t, c.expected, result, c.item.URL.String())
	}
}
