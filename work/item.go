package work

import (
	"fmt"
	"github.com/rickb777/path"
	"net/url"
	"strings"
	"time"
)

// Item is comparable
type Item struct {
	URL       *url.URL
	StartTime time.Time
	Referrer  *url.URL
	Depth     int
	FilePath  path.Path // returned when the item is processed
}

func (it Item) ChangePath(newPath string) Item {
	u2 := *it.URL
	u2.Path = newPath
	it.URL = &u2
	return it
}

func (it Item) String() string {
	return fmt.Sprintf("%s (depth:%d)", it.URL.String(), it.Depth)
}

//-------------------------------------------------------------------------------------------------

type Refs []*url.URL

func (refs Refs) String() string {
	buf := &strings.Builder{}
	spacer := ""
	for _, ref := range refs {
		buf.WriteString(spacer)
		buf.WriteString(ref.Host)
		buf.WriteString(ref.Path)
		spacer = " "
	}
	return buf.String()
}

//-------------------------------------------------------------------------------------------------

type Result struct {
	Item
	StatusCode    int
	References    Refs
	Excluded      Refs
	ContentLength int64
	FileSize      int64
	Gzip          bool
}
