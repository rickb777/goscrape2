package config

import (
	"math"
	"net/http"
	"regexp"
	"time"

	"github.com/rickb777/goscrape2/images"
)

// Config contains the scraper configuration.
type Config struct {
	Includes []*regexp.Regexp
	Excludes []*regexp.Regexp

	Concurrency    int                 // number of concurrent downloads; default 1
	MaxDepth       int                 // download depth, 0 for unlimited
	ImageQuality   images.ImageQuality // image quality from 0 to 100%, 0 to disable reencoding
	RequestTimeout time.Duration       // overall time limit to process each http request
	ConnectTimeout time.Duration       // time limit for connecting to the origin server
	LoopDelay      time.Duration       // fixed value sleep time per request
	LaxAge         time.Duration       // added to origin server's expires timestamp
	Tries          int                 // download attempts, 0 for unlimited

	Directory string
	Username  string
	Password  string

	Cookies   []Cookie
	Header    http.Header
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

	if c.RequestTimeout < 0 {
		c.RequestTimeout = 0
	}

	if c.LoopDelay < 0 {
		c.LoopDelay = 0
	}

	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 30 * time.Second
	}

	if c.RequestTimeout <= c.ConnectTimeout {
		c.RequestTimeout = c.ConnectTimeout + time.Second
	}
}

// Cookie represents a cookie, it copies parts of the http.Cookie struct but changes
// the JSON marshaling to exclude empty fields.
type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`

	Expires *time.Time `json:"expires,omitempty"`
}

func MakeHeaders(headers []struct {
	Key   string
	Value string
}) http.Header {
	h := http.Header{}
	for _, v := range headers {
		h.Add(v.Key, v.Value)
	}
	return h
}
