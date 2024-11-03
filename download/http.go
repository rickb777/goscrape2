package download

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cornelk/gotokit/log"
)

var DownloadURL = func(ctx context.Context, d *Download, u *url.URL) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	if d.Config.UserAgent != "" {
		req.Header.Set("User-Agent", d.Config.UserAgent)
	}

	if d.Auth != "" {
		req.Header.Set("Authorization", d.Auth)
	}

	for key, values := range d.Config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	sleepFor := 5 * time.Second // give the server plenty of time to recover; this grows larger every retry

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < d.Config.Tries; i++ {
		resp, err = d.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending HTTP request: %w", err)
		}

		switch {
		// 1xx status codes are never returned
		// 3xx redirect status code - handled by http.Client (up to 10 redirections)

		// 5xx status code = server error - retry the specified number of times
		case resp.StatusCode >= 500:
			// retry logic continues below

		// 4xx status code = client error
		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("unhandled HTTP client error: status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))

		// 304 not modified - no further action
		case resp.StatusCode == http.StatusNotModified:
			return nil, nil

		// 2xx status code = success
		default:
			logger.Debug("GET",
				log.String("url", u.String()),
				log.Int("status", resp.StatusCode),
				log.String("Content-Type", resp.Header.Get("Content-Type")),
				log.String("Content-Length", resp.Header.Get("Content-Length")),
				log.String("Last-Modified", resp.Header.Get("Last-Modified")))
			return resp, nil
		}

		logger.Warn("HTTP server error",
			log.String("url", req.URL.String()),
			log.Int("status", resp.StatusCode),
			log.String("sleep", sleepFor.String()))

		time.Sleep(sleepFor)
		sleepFor = backoff(sleepFor)
	}

	if resp == nil {
		return nil, fmt.Errorf("%s response status unknown", u)
	}
	return nil, fmt.Errorf("%s response status %d %s", resp.Request.URL, resp.StatusCode, http.StatusText(resp.StatusCode))
}

func backoff(t time.Duration) time.Duration {
	const factor = 7
	const divisor = 4 // must be less than factor
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
