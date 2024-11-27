package scraper

import (
	"fmt"
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/rickb777/servefiles/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
)

func mustParseURL(s string) *url.URL {
	u, e := url.Parse(s)
	if e != nil {
		panic(e)
	}
	return u
}

func setup() {
	sync.OnceFunc(func() {
		if testing.Verbose() {
			opts := &slog.HandlerOptions{Level: slog.LevelWarn}
			opts.Level = slog.LevelDebug
			servefiles.Debugf = func(format string, v ...interface{}) { logger.Debug(fmt.Sprintf(format, v...)) }
			logger.Logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
		} else {
			logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		}
	})
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
		{item: mustParseURL("http://example.org/ok/wanted"), expected: true},
		{item: mustParseURL("http://example.org/ok/nottoodeep"), depth: 10, expected: true},
		{item: mustParseURL("http://example.org/ok/toodeep"), depth: 11, expected: false},
		{item: mustParseURL("http://example.org/oktoodeep"), depth: 12, expected: false},
		{item: mustParseURL("http://example.org/other"), depth: 1, expected: false},
		{item: mustParseURL("ftp://example.org/ok"), expected: false},
		{item: mustParseURL("https://example.org/ok/done"), expected: false},
		{item: mustParseURL("https://other.org/ok"), expected: false},
		{item: mustParseURL("https://example.org/ok/bad"), expected: false},
	}

	for _, c := range cases {
		result := scraper.shouldURLBeDownloaded(c.item, c.depth)
		assert.Equal(t, c.expected, result, c.item.String())
	}
}
