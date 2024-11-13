package download

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cornelk/goscrape/utc"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/logger"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
)

// Counters accumulates HTTP response status codes.
var Counters = NewHistogram()

// httpGet performs one HTTP 'get' request, with as many retries as needed, up to the
// configured limit. Unless an error arises, the response body must be fully
// consumed and then closed.
func (d *Download) httpGet(ctx context.Context, u *url.URL, lastModified time.Time) (resp *http.Response, err error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set(headername.AcceptEncoding, "gzip")

	if d.Config.UserAgent != "" {
		req.Header.Set(headername.UserAgent, d.Config.UserAgent)
	}

	if d.Auth != "" {
		req.Header.Set(headername.Authorization, d.Auth)
	}

	// lastModified is only set when a locally-cached file exists
	if !lastModified.IsZero() {
		req.Header.Set(headername.IfModifiedSince, lastModified.Format(header.RFC1123))

		metadata := d.ETagsDB.Lookup(u)
		if d.Config.LaxAge >= 0 {
			now := utc.Now()
			if now.Before(metadata.Expires.Add(d.Config.LaxAge)) ||
				now.Before(lastModified.Add(d.Config.LaxAge)) {
				// not yet expired so no need for any HTTP traffic - report as 'teapot'
				return &http.Response{
					Request:       req,
					Status:        http.StatusText(http.StatusTeapot),
					StatusCode:    http.StatusTeapot, // treated like StatusNotModified
					Header:        http.Header{},
					Body:          io.NopCloser(&bytes.Buffer{}),
					ContentLength: 0,
				}, nil
			}
		}

		if len(metadata.ETags) > 0 {
			req.Header.Set(headername.IfNoneMatch, metadata.ETags)
		}
	}

	for key, values := range d.Config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	tries := d.Config.Tries
	if tries < 1 {
		tries = 1
	}

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < tries; i++ {
		time.Sleep(d.Config.LoopDelay) // fixed rate limiter
		d.Throttle.Sleep()             // variable throttle every URL

		resp, err = d.Client.Do(req)
		if err != nil {
			// halt the application
			return nil, fmt.Errorf("sending HTTP GET %s: %w", u, err)
		}

		Counters.Increment(resp.StatusCode)
		args := []any{slog.String("url", u.String()), slog.Int("status", resp.StatusCode)}
		args = addHeaderValue(args, resp.Header, headername.ContentType)
		args = addHeaderValue(args, resp.Header, headername.ContentLength)
		args = addHeaderValue(args, resp.Header, headername.LastModified)
		args = addHeaderValue(args, resp.Header, headername.ContentEncoding)
		args = addHeaderValue(args, resp.Header, headername.Vary)
		logger.Debug(http.MethodGet, args...)

		switch {
		// 1xx status codes are never returned
		// 3xx redirect status code - handled by http.Client (up to 10 redirections)

		// 5xx status code = server error - retry the specified number of times
		case resp.StatusCode >= 500:
			d.Throttle.SlowDown() // throttle every URL
			// retry logic continues below

		case resp.StatusCode == http.StatusTooManyRequests:
			d.Throttle.SlowDown() // throttle every URL
			return resp, nil      // this URL will be re-tried later

		// 4xx status code = client error
		case resp.StatusCode >= 400:
			d.Throttle.Reset()
			// returning no error allows ongoing downloading of other URLs
			return resp, nil // this url will be logged then discarded

		// 304 not modified - no download but scan for links if possible
		case resp.StatusCode == http.StatusNotModified:
			d.Throttle.Reset()
			return resp, nil

		// 2xx status code = success
		case 200 <= resp.StatusCode && resp.StatusCode < 300:
			d.Throttle.Reset()
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
