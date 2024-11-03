package config

import (
	"net/http"
	"strings"
	"time"
)

// Config contains the scraper configuration.
type Config struct {
	URL      string
	Includes []string
	Excludes []string

	ImageQuality uint          // image quality from 0 to 100%, 0 to disable reencoding
	MaxDepth     uint          // download depth, 0 for unlimited
	Timeout      time.Duration // time limit to process each http request
	Tries        int           // download attempts, 0 for unlimited

	OutputDirectory string
	Username        string
	Password        string

	Cookies   []Cookie
	Header    http.Header
	Proxy     string
	UserAgent string
}

// Cookie represents a cookie, it copies parts of the http.Cookie struct but changes
// the JSON marshaling to exclude empty fields.
type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`

	Expires *time.Time `json:"expires,omitempty"`
}

func MakeHeaders(headers []string) http.Header {
	h := http.Header{}
	for _, header := range headers {
		sl := strings.SplitN(header, ":", 2)
		if len(sl) == 2 {
			h.Set(sl[0], sl[1])
		}
	}
	return h
}
