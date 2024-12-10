package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/utc"
)

// Counters accumulates HTTP response status codes.
var Counters = NewHistogram()

// httpGet performs one HTTP 'get' request, with as many retries as needed, up to the
// configured limit.
//
// Unless an error arises, the response body must be fully consumed and closed by the caller.
func (d *Download) httpGet(ctx context.Context, u *url.URL, lastModified time.Time, metadata db.Item) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	// lastModified is only set when a locally-cached file exists
	if !lastModified.IsZero() {
		req.Header.Set(headername.IfModifiedSince, lastModified.Format(header.RFC1123))

		if d.Config.LaxAge >= 0 {
			now := utc.Now()
			if now.Before(metadata.Expires.Add(d.Config.LaxAge)) ||
				now.Before(lastModified.Add(d.Config.LaxAge)) {
				// not yet expired so no need for any HTTP traffic - report as 'teapot'
				return &http.Response{
					Request:       req,
					Status:        "Not Yet Expired",
					StatusCode:    http.StatusTeapot, // treated like StatusNotModified
					Header:        http.Header{},
					Body:          io.NopCloser(&bytes.Buffer{}),
					ContentLength: 0,
				}, nil
			}
		}
	}

	if len(metadata.ETags) > 0 {
		req.Header.Set(headername.IfNoneMatch, metadata.ETags)
	}

	req.Header.Set(headername.AcceptEncoding, "gzip")

	if d.Config.UserAgent != "" {
		req.Header.Set(headername.UserAgent, d.Config.UserAgent)
	}

	if d.Auth != "" {
		req.Header.Set(headername.Authorization, d.Auth)
	}

	for key, values := range d.Config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	return d.doHttpGet(req)
}

//-------------------------------------------------------------------------------------------------

func (d *Download) doHttpGet(req *http.Request) (resp *http.Response, err error) {
	tries := d.Config.Tries
	if tries < 1 {
		tries = 1
	}

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < tries; i++ {
		d.LoopDelay.Sleep() // mild rate limiter
		d.Lockdown.Sleep()  // severe rate limiter during 429 lockdown

		resp, err = d.Client.Do(req)
		if err != nil {
			return nil, err
		}

		Counters.Increment(resp.StatusCode)

		args := []any{slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode)}
		args = addHeaderValue(args, resp.Header, headername.ContentType)
		args = addHeaderValue(args, resp.Header, headername.ContentLength)
		args = addHeaderValue(args, resp.Header, headername.LastModified)
		args = addHeaderValue(args, resp.Header, headername.ContentEncoding)
		args = addHeaderValue(args, resp.Header, headername.Vary)
		logger.Debug(http.MethodGet, args...)

		switch {
		// 1xx status codes are never returned

		case resp.StatusCode == http.StatusTooManyRequests:
			d.Lockdown.SlowDown()  // back off request rate whilst we're being throttled by the server
			d.LoopDelay.SlowDown() // never return to the original speed
			return resp, nil       // this URL will be re-tried later

		// 304 not modified - no download but scan for links if possible
		case resp.StatusCode == http.StatusNotModified:
			d.Lockdown.Reset()
			return resp, nil

		// 5xx status code = server error - retry the specified number of times
		case resp.StatusCode >= 500:
			d.Lockdown.SlowDown() // back off request rate whilst the server is abnormal
			// retry logic continues below

		// 4xx status code = client error
		case resp.StatusCode >= 400:
			d.Lockdown.Reset()
			// returning no error allows ongoing downloading of other URLs
			return resp, nil // this url will be logged then discarded

		// 2xx status code = success
		// 3xx status code = redirect assumed
		case 200 <= resp.StatusCode && resp.StatusCode < 400:
			d.Lockdown.Reset()
			return resp, nil

		default:
			// halt the application
			return nil, fmt.Errorf("unexpected HTTP response %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		if i+1 < tries {
			logger.Warn(http.StatusText(resp.StatusCode),
				slog.String("url", req.URL.String()),
				slog.Int("code", resp.StatusCode))
		}
	}

	return resp, nil // allow this URL to be abandoned
}

//-------------------------------------------------------------------------------------------------

func closeResponseBody(c io.Closer, u *url.URL) {
	if err := c.Close(); err != nil {
		logger.Error("Closing HTTP response body failed",
			slog.Any("url", u),
			slog.Any("error", err))
	}
}

func addHeaderValue(args []any, header http.Header, name string) []any {
	value := header.Get(name)
	if value != "" {
		args = append(args, slog.String(name, value))
	}
	return args
}
