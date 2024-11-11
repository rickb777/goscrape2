package download

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
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

	if !lastModified.IsZero() {
		req.Header.Set(headername.IfModifiedSince, lastModified.Format(header.RFC1123))
	}

	etags := d.ETagsDB.Lookup(u)
	if len(etags) > 0 {
		req.Header.Set(headername.IfNoneMatch, etags.String())
	}

	for key, values := range d.Config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	retryDelay := d.Config.RetryDelay // used every retry for this URL only

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < d.Config.Tries; i++ {
		d.Throttle.Sleep() // throttle every URL

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
			d.Throttle.SpeedUp()
			retryDelay = backoff(retryDelay)
			// retry logic continues below

		case resp.StatusCode == http.StatusTooManyRequests:
			d.Throttle.Sleep() // throttle every URL
			return resp, nil   // this URL will be re-tried later

		// 4xx status code = client error
		case resp.StatusCode >= 400:
			d.Throttle.SpeedUp()
			logger.Error(http.StatusText(resp.StatusCode), slog.String("url", u.String()), slog.Int("code", resp.StatusCode))
			return resp, nil // no error allows ongoing downloading of other URLs

		// 304 not modified - no download but scan for links if possible
		case resp.StatusCode == http.StatusNotModified:
			d.Throttle.SpeedUp()
			return resp, nil

		// 2xx status code = success
		case 200 <= resp.StatusCode && resp.StatusCode < 300:
			d.Throttle.SpeedUp()
			return resp, nil

		default:
			// halt the application
			return nil, fmt.Errorf("unexpected HTTP response %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		logger.Warn("HTTP server error",
			slog.String("url", req.URL.String()),
			slog.Int("code", resp.StatusCode),
			slog.String("status", http.StatusText(resp.StatusCode)),
			slog.String("sleep", retryDelay.String()))

		if i+1 < d.Config.Tries {
			time.Sleep(retryDelay) // delay before retrying (this URL only)
		}
	}

	return resp, nil // allow this URL to be abandoned
}

func backoff(t time.Duration) time.Duration {
	const factor = 7
	const divisor = 4 // must be less than factor

	return time.Duration(t*factor) / divisor
}

func closeResponseBody(c io.Closer, u *url.URL) {
	if err := c.Close(); err != nil {
		logger.Error("Closing HTTP response body failed",
			slog.Any("url", u),
			slog.Any("error", err))
	}
}

func bufferEntireResponse(resp *http.Response, isGzip bool) ([]byte, error) {
	buf := &bytes.Buffer{}
	var err error

	rdr := resp.Body
	if isGzip {
		rdr, err = gzip.NewReader(rdr)
		if err != nil {
			logger.Error("Decompressing gzip response failed",
				slog.Any("url", resp.Request.URL),
				slog.Any("error", err))
			return nil, err
		}
		defer rdr.Close() // this only closes the gzipper, not the response body
	}

	if _, err := io.Copy(buf, rdr); err != nil {
		return nil, fmt.Errorf("%s reading response body: %w", resp.Request.URL, err)
	}
	return buf.Bytes(), nil
}

func addHeaderValue(args []any, header http.Header, name string) []any {
	value := header.Get(name)
	if value != "" {
		args = append(args, slog.String(name, value))
	}
	return args
}
