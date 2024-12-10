package download

import (
	"context"
	"github.com/rickb777/goscrape2/download/throttle"
	"github.com/rickb777/goscrape2/stubclient"
	"github.com/rickb777/goscrape2/utc"
	"github.com/spf13/afero"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet200(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client: stub,
		Auth:   "credentials",
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), lastModified, db.Item{})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.Equal(t, "Hello", resp.Request.Header.Get("X-Extra"))
}

func TestGet404(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusNotFound, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
		},
		Client: stub,
		Auth:   "credentials",
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), lastModified, db.Item{})

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
}

func TestGet429(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusTooManyRequests, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
		},
		Client:   stub,
		Auth:     "credentials",
		Lockdown: throttle.New(0, 10, 10),
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), lastModified, db.Item{})

	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.False(t, d.Lockdown.IsNormal())
}

func TestGet200RevalidateWhenExpired(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "http://example.org/", "text/html", `<html></html>`)

	u := mustParse("http://example.org/")
	item := db.Item{ETags: `"hash"`, Expires: utc.Now().Add(-time.Hour)}
	stub.Metadata.Store(u, item) // expired

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client: stub,
		Auth:   "credentials",
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), u, lastModified, item)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.Equal(t, "Hello", resp.Request.Header.Get("X-Extra"))
}

func TestGet200RevalidateWhenLaxIsNegative(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "http://example.org/", "text/html", `<html></html>`)

	u := mustParse("http://example.org/")
	item := db.Item{ETags: `"hash"`, Expires: utc.Now().Add(time.Hour)}
	stub.Metadata.Store(u, item) // not expired

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
			LaxAge:    -1,
		},
		Client: stub,
		Auth:   "credentials",
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), u, lastModified, item)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.Equal(t, "Hello", resp.Request.Header.Get("X-Extra"))
}

func TestGet304UsingEtag(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "http://example.org/", "text/html", `<html></html>`, header.ETag{Hash: "hash"})

	stub.Metadata = db.OpenDB(".", afero.NewMemMapFs())
	defer os.Remove("./" + db.FileName)
	defer stub.Metadata.Close()

	u := mustParse("http://example.org/")
	item := db.Item{Code: 200, ETags: `"hash"`}
	stub.Metadata.Store(u, item)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client:  stub,
		Auth:    "credentials",
		ETagsDB: stub.Metadata,
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), u, lastModified, item)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotModified, resp.StatusCode)
}

func TestNotYetExpired(t *testing.T) {
	stub := &stubclient.Client{}

	stub.Metadata = db.OpenDB(".", afero.NewMemMapFs())
	defer os.Remove("./" + db.FileName)
	defer stub.Metadata.Close()

	u := mustParse("http://example.org/")
	item := db.Item{Code: 200, ETags: `"hash"`, Expires: utc.Now().Add(time.Hour)}
	stub.Metadata.Store(u, item) // not expired

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client:  stub,
		Auth:    "credentials",
		ETagsDB: stub.Metadata,
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), u, lastModified, item)

	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
}

func TestGet500(t *testing.T) {
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusInternalServerError, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			Tries: 2,
		},
		Client: stub,
	}

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), time.Time{}, db.Item{})

	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.Equal(t, "", resp.Request.Header.Get("X-Extra"))
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
