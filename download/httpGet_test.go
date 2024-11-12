package download

import (
	"bytes"
	"context"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/db"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubClient struct {
	responses map[string]http.Response // more configurable responses
	eTags     *db.DB
}

func (c *stubClient) response(statusCode int, url, contentType, body string, etags ...header.ETag) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	rdr := bytes.NewReader([]byte(body))
	resp := http.Response{
		Request:    req,
		Header:     http.Header{headername.ContentType: []string{contentType}},
		Body:       io.NopCloser(rdr),
		StatusCode: statusCode,
	}
	if len(etags) > 0 {
		resp.Header.Set("ETag", header.ETags(etags).String())
	}
	if c.responses == nil {
		c.responses = make(map[string]http.Response)
	}
	c.responses[url] = resp
}

func (c *stubClient) Do(req *http.Request) (resp *http.Response, err error) {
	ur := req.URL.String()
	r, ok := c.responses[ur]
	if !ok {
		panic(fmt.Sprintf("url '%s' not found in test data", ur))
	}

	metadata := c.eTags.Lookup(req.URL)
	if len(metadata.ETags) > 0 && r.StatusCode == http.StatusOK {
		wanted := header.ETagsOf(req.Header.Get(headername.IfNoneMatch))
		for _, w := range wanted {
			if header.ETagsOf(metadata.ETags).WeaklyMatches(w.Hash) {
				r.StatusCode = http.StatusNotModified
				r.Status = http.StatusText(http.StatusNotModified)
				r.Body = io.NopCloser(&bytes.Buffer{})
				break
			}
		}
	}

	r.Request = req
	return &r, nil
}

//-------------------------------------------------------------------------------------------------

func TestGet200(t *testing.T) {
	stub := &stubClient{}
	stub.response(http.StatusOK, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			Tries:     1,
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client: stub,
		Auth:   "credentials",
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), lastModified)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Request.Header.Get(headername.AcceptEncoding))
	assert.Equal(t, "Foo/Bar", resp.Request.Header.Get(headername.UserAgent))
	assert.Equal(t, "Sat, 01 Jan 2000 01:01:01 UTC", resp.Request.Header.Get(headername.IfModifiedSince))
	assert.Equal(t, "Hello", resp.Request.Header.Get("X-Extra"))
}

func TestGet304UsingEtag(t *testing.T) {
	stub := &stubClient{}
	stub.response(http.StatusOK, "http://example.org/", "text/html", `<html></html>`, header.ETag{Hash: "hash"})

	stub.eTags = db.OpenDB(".", afero.NewMemMapFs())
	defer os.Remove("./" + db.FileName)
	defer stub.eTags.Close()

	u := mustParse("http://example.org/")
	stub.eTags.Store(u, db.Item{ETags: `"hash"`})

	d := &Download{
		Config: config.Config{
			Tries:     1,
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client:  stub,
		Auth:    "credentials",
		ETagsDB: stub.eTags,
	}

	lastModified := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)

	resp, err := d.httpGet(context.Background(), u, lastModified)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotModified, resp.StatusCode)
}

func TestGet500(t *testing.T) {
	stub := &stubClient{}
	stub.response(http.StatusInternalServerError, "http://example.org/", "text/html", `<html></html>`)

	d := &Download{
		Config: config.Config{
			Tries: 2,
		},
		Client: stub,
	}

	resp, err := d.httpGet(context.Background(), mustParse("http://example.org/"), time.Time{})

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
