package download

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/download/throttle"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/mapping"
	"github.com/cornelk/goscrape/utc"
	"github.com/cornelk/goscrape/work"
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
	var existingModified time.Time

	item.FilePath = mapping.GetFilePath(item.URL, true)

	fileInfo, err := d.Fs.Stat(item.FilePath)
	if err == nil && fileInfo != nil {
		existingModified = fileInfo.ModTime()
	}

	item.StartTime = utc.Now()

	resp, err := d.httpGet(ctx, item.URL, existingModified)
	if err != nil {
		logger.Error("Processing HTTP Request failed",
			slog.String("url", item.URL.String()),
			slog.Any("error", err))
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
func (d *Download) response429(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	// put this URL back into the work queue to be re-tried later
	repeat := &work.Result{Item: item, StatusCode: http.StatusTooManyRequests, References: []*url.URL{item.URL}}
	repeat.Item.Depth-- // because it will get incremented and we need the retry depth to be unchanged
	return item.URL, repeat, nil
}

//-------------------------------------------------------------------------------------------------

func discardData(rdr io.Reader) {
	// Consume any response body - necessary for correct operation of the TCP connection pool
	_, _ = io.Copy(io.Discard, rdr)
}
