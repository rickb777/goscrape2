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

func (it Item) Derive(u *url.URL) Item {
	return Item{
		URL:      u,
		Depth:    it.Depth + 1,
		Referrer: it.URL,
	}
}
