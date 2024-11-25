package scraper

import (
	"context"
	"github.com/cornelk/goscrape/stubclient"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
)

func TestServeDirectory(t *testing.T) {
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
	//originStub.GivenResponse(http.StatusOK, "https://example.org/", "text/html", indexPage)
	//originStub.GivenResponse(http.StatusOK, "https://example.org/page2/", "text/html", page2)
	//originStub.GivenResponse(http.StatusOK, "https://example.org/sub/", "text/html", subPage)
	//originStub.GivenResponse(http.StatusOK, "https://example.org/style.css", "text/css", "")
	originStub.GivenResponse(http.StatusOK, "https://example.org/missing.html", "text/html", missing)

	scraper := newTestScraper(t, startURL, originStub)
	require.NotNil(t, scraper)

	fs := afero.NewMemMapFs()
	writeFile(fs, "index.html", indexPage)
	writeFile(fs, "page2/index.html", page2)
	writeFile(fs, "sub/index.html", subPage)
	writeFile(fs, "style.css", "{}")

	server, err := newWebserver(fs, 14141, scraper)
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
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	//resp, err = c.Get("http://localhost:14141/page2/")
	//require.NoError(t, err)
	//assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = c.Get("http://localhost:14141/missing.html")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

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
