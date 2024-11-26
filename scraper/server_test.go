package scraper

import (
	"context"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/rickb777/servefiles/v3"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func setup() {
	if testing.Verbose() {
		sync.OnceFunc(func() {
			opts := &slog.HandlerOptions{Level: slog.LevelWarn}
			opts.Level = slog.LevelDebug
			servefiles.Debugf = func(format string, v ...interface{}) { logger.Debug(fmt.Sprintf(format, v...)) }
			logger.Logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
		})
	}
}

func TestServeDirectory(t *testing.T) {
	setup()
	startURL := "https://example.org/"

	stub := &stubclient.Client{}
	//stub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", indexPage)
	//stub.GivenResponse(http.StatusOK, "https://example.org/bg.gif", "image/gif", "")

	scraper := newTestScraper(t, startURL, stub)
	require.NotNil(t, scraper)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := ServeDirectory(ctx, "", 14141, scraper)
	require.NoError(t, err)
}

func TestWebserver(t *testing.T) {
	setup()
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

	startURL := "https://example.org/"

	originStub := &stubclient.Client{}
	originStub.GivenResponse(http.StatusOK, "https://example.org/missing.html", "text/html", missing)

	scraper := newTestScraper(t, startURL, originStub)
	require.NotNil(t, scraper)

	scraper.fs = afero.NewMemMapFs()
	writeFile(scraper.fs, "example.org/index.html", indexPage)
	writeFile(scraper.fs, "example.org/page2/index.html", page2)
	writeFile(scraper.fs, "example.org/sub/index.html", subPage)
	writeFile(scraper.fs, "example.org/style.css", "{}")

	server, err := newWebserver(14141, nil, scraper)
	require.NoError(t, err)
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
