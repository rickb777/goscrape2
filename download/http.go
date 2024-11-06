package download

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rickb777/acceptable/headername"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
)

const (
	initialRetryDelay = 5 * time.Second
	bigRetryDelay     = 30 * time.Second
)

// Errors4xx accumulates 4xx HTTP errors.
var Errors4xx = NewHistogram()

func (d *Download) GET(ctx context.Context, u *url.URL) (resp *http.Response, err error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

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

	retryDelay := initialRetryDelay // used every retry for this URL only

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < d.Config.Tries; i++ {
		d.Throttle.Sleep() // throttle every URL

		resp, err = d.Client.Do(req)
		if err != nil {
			// halt the application
			return nil, fmt.Errorf("sending HTTP GET %s: %w", u, err)
		}

		switch {
		// 1xx status codes are never returned
		// 3xx redirect status code - handled by http.Client (up to 10 redirections)

		// 5xx status code = server error - retry the specified number of times
		case resp.StatusCode >= 500:
			retryDelay = backoff(retryDelay)
			// retry logic continues below

		case resp.StatusCode == http.StatusTooManyRequests:
			d.Throttle.SlowDown() // affects all URLs
			d.Throttle.Sleep()    // throttle every URL
			return resp, nil      // this URL will be re-tried later

		// 4xx status code = client error
		case resp.StatusCode >= 400:
			Errors4xx.Increment(resp.StatusCode)
			logger.Error("HTTP client error", log.String("url", u.String()),
				log.Int("code", resp.StatusCode), log.String("status", http.StatusText(resp.StatusCode)))
			return nil, nil // no error allows ongoing downloading

		// 304 not modified - no further action
		case resp.StatusCode == http.StatusNotModified:
			return nil, nil

		// 2xx status code = success
		case 200 <= resp.StatusCode && resp.StatusCode < 300:
			d.Throttle.SpeedUp()
			logger.Debug(http.MethodGet,
				log.String("url", u.String()),
				log.Int("status", resp.StatusCode),
				log.String(headername.ContentType, resp.Header.Get(headername.ContentType)),
				log.String(headername.ContentLength, resp.Header.Get(headername.ContentLength)),
				log.String(headername.LastModified, resp.Header.Get(headername.LastModified)))
			return resp, nil

		default:
			// halt the application
			return nil, fmt.Errorf("unexpected HTTP response %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		logger.Warn("HTTP server error",
			log.String("url", req.URL.String()),
			log.Int("code", resp.StatusCode),
			log.String("status", http.StatusText(resp.StatusCode)),
			log.String("sleep", retryDelay.String()))

		time.Sleep(retryDelay)
	}

	if resp == nil {
		return nil, fmt.Errorf("%s response status unknown", u)
	}
	return nil, fmt.Errorf("%s response status %d %s", resp.Request.URL, resp.StatusCode, http.StatusText(resp.StatusCode))
}

func backoff(t time.Duration) time.Duration {
	const factor = 7
	const divisor = 4 // must be less than factor

	if t < initialRetryDelay {
		return initialRetryDelay
	}

	return time.Duration(t*factor) / divisor
}

func closeResponseBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		logger.Error("Closing HTTP response body failed",
			log.String("url", resp.Request.URL.String()),
			log.Err(err))
	}
}

func bufferEntireResponse(resp *http.Response) ([]byte, error) {
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, fmt.Errorf("%s reading response body: %w", resp.Request.URL, err)
	}
	return buf.Bytes(), nil
}
