package download

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/rickb777/acceptable/headername"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/download/throttle"
	"github.com/rickb777/goscrape2/logger"
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
		d.ETagsDB.Store(item.URL, db.Item{Code: resp.StatusCode, Expires: utc.Now().Add(d.Config.GetLaxAge())})
		return item.URL, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil

	case http.StatusForbidden, http.StatusGone, http.StatusUnavailableForLegalReasons:
		discardData(resp.Body) // discard anything present
		return d.responseGone(item, resp)

	case http.StatusTooManyRequests:
		discardData(resp.Body) // discard anything present
		return d.response429(item, resp)

	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		discardData(resp.Body) // discard anything present
		return d.responseRedirect(item, resp)

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

// responseRedirect handles redirection
func (d *Download) responseRedirect(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	location := resp.Header.Get(headername.Location)

	locURL, err := url.Parse(location)
	if err != nil {
		logger.Error("Parse location header failed",
			slog.Any("url", item.URL),
			slog.Int("code", resp.StatusCode),
			slog.String("location", location),
			slog.Any("error", err))
		return nil, nil, err
	}

	// store so that the webserver can replay the redirect
	d.ETagsDB.Store(item.URL, db.Item{Code: resp.StatusCode, Location: location})

	// put this URL back into the work queue to be processed later
	item.FilePath = ""
	redirect := &work.Result{Item: item, StatusCode: resp.StatusCode, References: []*url.URL{locURL}}
	redirect.Item.Depth-- // because it will get incremented and we need the redirect depth to be unchanged
	return item.URL, redirect, nil
}

//-------------------------------------------------------------------------------------------------

// response429 handles too-many-request responses.
func (d *Download) response429(item work.Item, _ *http.Response) (*url.URL, *work.Result, error) {
	// put this URL back into the work queue to be re-tried later
	item.FilePath = ""
	repeat := &work.Result{Item: item, StatusCode: http.StatusTooManyRequests, References: []*url.URL{item.URL}}
	repeat.Item.Depth-- // because it will get incremented and we need the retry depth to be unchanged
	return item.URL, repeat, nil
}

//-------------------------------------------------------------------------------------------------

func discardData(rdr io.Reader) {
	// Consume any response body - necessary for correct operation of the TCP connection pool
	_, _ = io.Copy(io.Discard, rdr)
}
