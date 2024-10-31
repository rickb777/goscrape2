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

func (s *Scraper) downloadURL(ctx context.Context, u *url.URL) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", s.config.UserAgent)
	if s.auth != "" {
		req.Header.Set("Authorization", s.auth)
	}

	for key, values := range s.config.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	var statusCode int
	var body []byte
	var actualUrl *url.URL
	sleepFor := 5 * time.Second // give the server plenty of time to recover

	// this loop provides retries if 5xx server errors arise
	for i := 0; i < s.config.Tries; i++ {
		statusCode, body, actualUrl, err = s.makeHttpRequest(req)
		if err != nil {
			return body, actualUrl, err
		}

		switch statusCode / 100 {
		// 1xx status codes are never returned
		// 3xx status code = redirect - handled by http.Client (up to 10 redirections)
		// 5xx status code = server error - retry the specified number of times

		case 2: // 2xx status code = success
			return body, actualUrl, err
		case 4: // 4xx status code = client error
			return nil, nil, fmt.Errorf("unexpected HTTP response status code %d", statusCode)
		}

		s.logger.Warn("HTTP server error",
			log.String("url", req.URL.String()),
			log.Int("status", statusCode))
		log.String("sleep", sleepFor.String())

		time.Sleep(sleepFor)
		sleepFor = backoff(sleepFor)
	}

	return nil, nil, fmt.Errorf("unexpected HTTP response status code %d", statusCode)
}

func backoff(t time.Duration) time.Duration {
	const factor = 7
	const divisor = 4 // must be less than factor
	return time.Duration(t*factor) / divisor
}

func (s *Scraper) makeHttpRequest(req *http.Request) (int, []byte, *url.URL, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.logger.Error("Closing HTTP Request body failed",
				log.String("url", req.URL.String()),
				log.Err(err))
		}
	}()

	// All 2xx responses are considered acceptable, although some may have no response body
	if 200 <= resp.StatusCode && resp.StatusCode < 300 {
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, resp.Body); err != nil {
			return 0, nil, nil, fmt.Errorf("reading HTTP response body: %w", err)
		}
		return resp.StatusCode, buf.Bytes(), resp.Request.URL, nil
	}

	return resp.StatusCode, nil, nil, nil
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
