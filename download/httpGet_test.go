package download

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/download/throttle"
	"github.com/rickb777/goscrape2/stubclient"
	"github.com/rickb777/goscrape2/utc"
	"github.com/spf13/afero"
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusOK)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "Foo/Bar")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "Sat, 01 Jan 2000 01:01:01 UTC")
	expect.String(resp.Request.Header.Get("X-Extra")).ToBe(t, "Hello")
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusNotFound)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "Foo/Bar")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "Sat, 01 Jan 2000 01:01:01 UTC")
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusTooManyRequests)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "Foo/Bar")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "Sat, 01 Jan 2000 01:01:01 UTC")
	expect.Bool(d.Lockdown.IsNormal()).ToBeFalse(t)
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusOK)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "Foo/Bar")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "Sat, 01 Jan 2000 01:01:01 UTC")
	expect.String(resp.Request.Header.Get("X-Extra")).ToBe(t, "Hello")
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusOK)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "Foo/Bar")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "Sat, 01 Jan 2000 01:01:01 UTC")
	expect.String(resp.Request.Header.Get("X-Extra")).ToBe(t, "Hello")
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusNotModified)
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusTeapot)
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

	expect.Error(err).ToBeNil(t)
	expect.Number(resp.StatusCode).ToBe(t, http.StatusInternalServerError)
	expect.String(resp.Request.Header.Get(headername.AcceptEncoding)).ToBe(t, "gzip")
	expect.String(resp.Request.Header.Get(headername.UserAgent)).ToBe(t, "")
	expect.String(resp.Request.Header.Get(headername.IfModifiedSince)).ToBe(t, "")
	expect.String(resp.Request.Header.Get("X-Extra")).ToBe(t, "")
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
