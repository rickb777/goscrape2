package config

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/cornelk/goscrape/images"
)

// Config contains the scraper configuration.
type Config struct {
	Includes []string
	Excludes []string

	Concurrency  int                 // number of concurrent downloads; default 1
	MaxDepth     int                 // download depth, 0 for unlimited
	ImageQuality images.ImageQuality // image quality from 0 to 100%, 0 to disable reencoding
	Timeout      time.Duration       // time limit to process each http request
	LoopDelay    time.Duration       // fixed value sleep time per request
	LaxAge       time.Duration       // added to origin server's expires timestamp
	Tries        int                 // download attempts, 0 for unlimited

	Directory string
	Username  string
	Password  string

	Cookies   []Cookie
	Header    http.Header
	Proxy     string
	UserAgent string
}

func (c *Config) GetLaxAge() time.Duration {
	if c.LaxAge > 0 {
		return c.LaxAge
	}
	return 0
}

func (c *Config) SensibleDefaults() {
	if c.Concurrency < 1 {
		c.Concurrency = 1
	}

	if c.Tries < 1 {
		c.Tries = 1
	}

	if c.MaxDepth < 1 {
		c.MaxDepth = math.MaxInt
	}

	if c.Timeout < 0 {
		c.Timeout = 0
	}

	if c.LoopDelay < 0 {
		c.LoopDelay = 0
	}
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
