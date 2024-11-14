package stubclient

import (
	"bytes"
	"fmt"
	"github.com/cornelk/goscrape/db"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"io"
	"net/http"
)

// Client is for http testing.
type Client struct {
	responses map[string]http.Response // more configurable responses
	Metadata  *db.DB
}

func (c *Client) GivenResponse(statusCode int, url, contentType, body string, etags ...header.ETag) {
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

func (c *Client) Do(req *http.Request) (resp *http.Response, err error) {
	ur := req.URL.String()
	r, ok := c.responses[ur]
	if !ok {
		panic(fmt.Sprintf("url '%s' not found in test data", ur))
	}

	metadata := c.Metadata.Lookup(req.URL)
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
