package scraper

import (
	"fmt"
	"github.com/cornelk/goscrape/config"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

// Cookies returns the current cookies.
func (s *Scraper) Cookies() []config.Cookie {
	httpCookies := s.cookies.Cookies(s.URL)
	cookies := make([]config.Cookie, 0, len(httpCookies))

	for _, c := range httpCookies {
		cookie := config.Cookie{
			Name:  c.Name,
			Value: c.Value,
		}
		if !c.Expires.IsZero() {
			cookie.Expires = &c.Expires
		}
		cookies = append(cookies, cookie)
	}

	return cookies
}

func createCookieJar(u *url.URL, cookies []config.Cookie) (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		h := &http.Cookie{
			Name:  c.Name,
			Value: c.Value,
		}
		if c.Expires != nil {
			h.Expires = *c.Expires
		}
		httpCookies = append(httpCookies, h)
	}

	jar.SetCookies(u, httpCookies)
	return jar, nil
}
