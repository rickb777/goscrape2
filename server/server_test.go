package server

import (
	"context"
	"fmt"
	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/scraper"
	"github.com/rickb777/goscrape2/stubclient"
	"github.com/rickb777/servefiles/v3"
	"github.com/spf13/afero"
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
	expect.Error(err).ToBeNil(t)
	expect.Any(sc).Not().ToBeNil(t)

	sc.Client = stub
	return sc
}

func TestServeDirectory(t *testing.T) {
	stub := &stubclient.Client{}
	sc := newTestScraper(t, "https://example.org/", stub)
	expect.Any(sc).Not().ToBeNil(t)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// not testing what it actually does here - see below
	err := ServeDirectory(ctx, sc, "", 14141)
	expect.Error(err).ToBeNil(t)
}

func TestLaunchWebserver_connectedToOrigin(t *testing.T) {
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
	expect.Any(sc).Not().ToBeNil(t)

	sc.Fs = afero.NewMemMapFs()
	writeFile(sc.Fs, "example.org/index.html", indexPage)
	writeFile(sc.Fs, "example.org/page2/index.html", page2)

	server, errChan, err := LaunchWebserver(sc, "", 14141)
	expect.Error(err).ToBeNil(t)
	expect.Any(server).Not().ToBeNil(t)

	c := &http.Client{}

	resp, err := c.Get("http://localhost:14141/")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/").ToBe(t, http.StatusOK)

	resp, err = c.Get("http://localhost:14141/missing.html")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/missing.html").ToBe(t, http.StatusOK)

	resp, err = c.Get("http://localhost:14141/page2/")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/page2/").ToBe(t, http.StatusOK)

	err = server.Shutdown(context.Background())
	expect.Error(err).ToBeNil(t)

	expect.Any(<-errChan).ToBe(t, http.ErrServerClosed)
}

func TestLaunchWebserver_notConnected(t *testing.T) {
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

	page2 := `
<html>
<body>

<a href="/">a</a>
<a href="/sub/">a</a>

</body>
</html>
`

	originStub := &stubclient.Client{}
	originStub.GivenError("https://example.org/missing.html",
		&url.Error{
			Op:  "Get",
			URL: "https://example.org/missing.html",
		})

	sc := newTestScraper(t, "https://example.org/", originStub)
	expect.Any(sc).Not().ToBeNil(t)

	sc.Fs = afero.NewMemMapFs()
	writeFile(sc.Fs, "example.org/index.html", indexPage)
	writeFile(sc.Fs, "example.org/page2/index.html", page2)

	server, errChan, err := LaunchWebserver(sc, "", 14141)
	expect.Error(err).ToBeNil(t)
	expect.Any(server).Not().ToBeNil(t)

	time.Sleep(50 * time.Millisecond)

	c := &http.Client{}

	resp, err := c.Get("http://localhost:14141/")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/").ToBe(t, http.StatusOK)

	resp, err = c.Get("http://localhost:14141/missing.html")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/missing.html").ToBe(t, http.StatusBadGateway)

	resp, err = c.Get("http://localhost:14141/page2/")
	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).I("/page2/").ToBe(t, http.StatusOK)

	err = server.Shutdown(context.Background())
	expect.Error(err).ToBeNil(t)

	expect.Any(<-errChan).ToBe(t, http.ErrServerClosed)
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
