package work

import (
	"fmt"
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

func (it Item) String() string {
	return fmt.Sprintf("%s (depth:%d)", it.URL.String(), it.Depth)
}
