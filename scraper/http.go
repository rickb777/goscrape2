package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cornelk/gotokit/log"
)

func (s *Scraper) downloadURL(ctx context.Context, u *url.URL) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	if s.config.UserAgent != "" {
		req.Header.Set("User-Agent", s.config.UserAgent)
	}

	if s.auth != "" {
		req.Header.Set("Authorization", s.auth)
	}

	for key, values := range s.config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	sleepFor := 5 * time.Second // give the server plenty of time to recover; this grows larger every retry

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < s.config.Tries; i++ {
		resp, err = s.client.Do(req)
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
			return resp, nil
		}

		s.logger.Warn("HTTP server error",
			log.String("url", req.URL.String()),
			log.Int("status", resp.StatusCode),
			log.String("sleep", sleepFor.String()))

		time.Sleep(sleepFor)
		sleepFor = backoff(sleepFor)
	}

	if resp == nil {
		return nil, fmt.Errorf("%s response status unknown", resp.Request.URL)
	}
	return nil, fmt.Errorf("%s response status %d %s", resp.Request.URL, resp.StatusCode, http.StatusText(resp.StatusCode))
}

func backoff(t time.Duration) time.Duration {
	const factor = 7
	const divisor = 4 // must be less than factor
	return time.Duration(t*factor) / divisor
}

func closeResponseBody(resp *http.Response, logger *log.Logger) {
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

func Headers(headers []string) http.Header {
	h := http.Header{}
	for _, header := range headers {
		sl := strings.SplitN(header, ":", 2)
		if len(sl) == 2 {
			h.Set(sl[0], sl[1])
		}
	}
	return h
}
