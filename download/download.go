package download

import (
	"context"
	"github.com/rickb777/acceptable/header"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/download/throttle"
	"github.com/rickb777/goscrape2/mapping"
	"github.com/rickb777/goscrape2/utc"
	"github.com/rickb777/goscrape2/work"
	"github.com/spf13/afero"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Download fetches URLs one by one, sequentially.
type Download struct {
	Config   config.Config
	Cookies  *cookiejar.Jar
	ETagsDB  *db.DB
	StartURL *url.URL

	Auth   string
	Client HttpClient
	Fs     afero.Fs // filesystem can be replaced with in-memory filesystem for testing

	Lockdown  *throttle.Throttle // increases sharply when server gives 429 (Too Many Requests) responses, then resets
	LoopDelay *throttle.Throttle // increases only slightly when server gives 429; never decreases
}

func (d *Download) ProcessURL(ctx context.Context, item work.Item) (*url.URL, *work.Result, error) {
	metadata := d.ETagsDB.Lookup(item.URL)

	//if isAmbiguousPath(item.URL.Path) {
	//	item2 := item.ChangePath(item.URL.Path + "/")
	//	m2 := d.ETagsDB.Lookup(item2.URL)
	//	if !m2.EmptyContentType() {
	//		// we know it existed earlier; go straight to item2 and skip original item
	//		return d.doProcessURL(ctx, item2, m2)
	//	} else {
	//		_, result2, err2 := d.doProcessURL(ctx, item2, m2)
	//		if err2 == nil && result2.StatusCode != http.StatusNotFound {
	//			return item2.URL, result2, nil
	//		}
	//	}
	//}

	var existingModified time.Time

	item.FilePath = mapping.GetFilePath(item.URL, true)

	fileInfo, err := d.Fs.Stat(item.FilePath)
	if err == nil && fileInfo != nil {
		existingModified = fileInfo.ModTime()
	}

	item.StartTime = utc.Now()

	resp, err := d.httpGet(ctx, item.URL, existingModified, metadata)
	if err != nil {
		return nil, nil, err
	}

	if resp == nil {
		panic("unexpected nil response")
	}

	// n.b. for correct connection pooling in the HTTP client, every response must
	// be fully consumed and closed
	defer closeResponseBody(resp.Body, resp.Request.URL)

	if item.Depth == 0 {
		// take account of redirection (only on the start page)
		item.URL = resp.Request.URL
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// write the response body to a file, possibly modifying its hyperlinks
		return d.response200(item, resp)

	case http.StatusNotModified, http.StatusTeapot:
		discardData(resp.Body) // discard anything present
		return d.response304(item, resp)

	case http.StatusNotFound:
		discardData(resp.Body) // discard anything present
		d.ETagsDB.Store(item.URL, db.Item{Expires: utc.Now().Add(d.Config.GetLaxAge())})
		return item.URL, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil

	case http.StatusForbidden, http.StatusGone, http.StatusUnavailableForLegalReasons:
		discardData(resp.Body) // discard anything present
		return d.responseGone(item, resp)

	case http.StatusTooManyRequests:
		discardData(resp.Body) // discard anything present
		return d.response429(item, resp)

	default:
		discardData(resp.Body) // didn't want it
		return item.URL, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil
	}
}

//-------------------------------------------------------------------------------------------------

// responseGone deletes obsolete/inaccessible files
func (d *Download) responseGone(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	filePath := mapping.GetFilePath(item.URL, true)
	_ = d.Fs.Remove(filePath)
	return item.URL, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil
}

//-------------------------------------------------------------------------------------------------

// response429 handles too-many-request responses.
func (d *Download) response429(item work.Item, _ *http.Response) (*url.URL, *work.Result, error) {
	// put this URL back into the work queue to be re-tried later
	repeat := &work.Result{Item: item, StatusCode: http.StatusTooManyRequests, References: []*url.URL{item.URL}}
	repeat.Item.Depth-- // because it will get incremented and we need the retry depth to be unchanged
	return item.URL, repeat, nil
}

//-------------------------------------------------------------------------------------------------

// isAmbiguousPath returns true if the path has a filename without any extension. It might
// be that the server is representing a directory without a trailing slash, which is a commonly
// used pattern.
func isAmbiguousPath(p string) bool {
	ext := path.Ext(p)
	return ext == "" && !strings.HasSuffix(p, "/")
}

func discardData(rdr io.Reader) {
	// Consume any response body - necessary for correct operation of the TCP connection pool
	_, _ = io.Copy(io.Discard, rdr)
}

var starStar = header.ContentType{Type: "*", Subtype: "*"}
