package server

import (
	"context"
	"fmt"
	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/scraper"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/rickb777/servefiles/v3"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
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

func newTestScraper(t *testing.T, startURL string, stub *stubclient.Client) *scraper.Scraper {
	setup()
	t.Helper()

	cfg := config.Config{MaxDepth: 10}
	sc, err := scraper.New(cfg, mustParseURL(startURL), afero.NewMemMapFs())
	require.NoError(t, err)
	require.NotNil(t, sc)

	sc.Client = stub
	return sc
}

func TestServeDirectory(t *testing.T) {
	stub := &stubclient.Client{}
	sc := newTestScraper(t, "https://example.org/", stub)
	require.NotNil(t, sc)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := ServeDirectory(ctx, "", 14141, sc)
	require.NoError(t, err)
}

func TestWebserver(t *testing.T) {
	indexPage := `
<html>
<head>
<link href='style.css' rel='stylesheet' type='text/css'>
</head>
<body>Index
<a href="page2">Example</a>
</body>
</html>
`

	subPage := `
<html>
<head>
<link href='../style.css' rel='stylesheet' type='text/css'>
</head>
<body>Sub
<a href="../page2">Example</a>
</body>
</html>
`

	page2 := `
<html>
<body>

<a href="/">a</a>
<a href="/sub/">a</a>

</body>
</html>
`

	missing := `
<html>
<body>
It's here!
</body>
</html>
`

	originStub := &stubclient.Client{}
	originStub.GivenResponse(http.StatusOK, "https://example.org/missing.html", "text/html", missing)

	sc := newTestScraper(t, "https://example.org/", originStub)
	require.NotNil(t, sc)

	sc.Fs = afero.NewMemMapFs()
	writeFile(sc.Fs, "example.org/index.html", indexPage)
	writeFile(sc.Fs, "example.org/page2/index.html", page2)
	writeFile(sc.Fs, "example.org/sub/index.html", subPage)
	writeFile(sc.Fs, "example.org/style.css", "{}")

	fileServer := assetHandlerWith404Handler(sc)
	server := newWebserver(14141, fileServer)
	require.NotNil(t, server)

	go func() {
		e2 := server.ListenAndServe()
		if e2 != http.ErrServerClosed {
			panic(e2)
		}
	}()

	c := &http.Client{}

	resp, err := c.Get("http://localhost:14141/")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "/")

	resp, err = c.Get("http://localhost:14141/page2/")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "/page2/")

	resp, err = c.Get("http://localhost:14141/missing.html")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "/missing.html")

	err = server.Shutdown(context.Background())
	require.NoError(t, err)
}

func writeFile(fs afero.Fs, name, text string) {
	f, err := fs.Create(name)
	must(err)
	defer f.Close()

	_, err = f.Write([]byte(text))
	must(err)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
