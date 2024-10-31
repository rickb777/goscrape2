package work

import (
	"net/url"
)

// Item is comparable
type Item struct {
	URL      *url.URL
	Referrer *url.URL
	Depth    uint
}
