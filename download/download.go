package download

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/document"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/logger"
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

	Fs       afero.Fs // filesystem can be replaced with in-memory filesystem for testing
	Throttle Throttle // increases when server gives 429 (Too Many Requests) responses
}

func (d *Download) ProcessURL(ctx context.Context, item work.Item) (*url.URL, *work.Result, error) {
	var existingModified time.Time

	filePath := document.GetFilePath(item.URL, d.StartURL, d.Config.OutputDirectory, true)
	if ioutil.FileExists(d.Fs, filePath) {
		fileInfo, err := os.Stat(filePath)
		if err == nil && fileInfo != nil {
			existingModified = fileInfo.ModTime()
		}
	}

	item.StartTime = time.Now()

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
		return d.response200(item, resp)

	case http.StatusNotModified:
		return d.response304(item, resp)

	case http.StatusTooManyRequests:
		return d.response429(item, resp)

	default:
		discardData(resp.Body) // didn't want it
		return item.URL, &work.Result{Item: item}, nil
	}
}

//-------------------------------------------------------------------------------------------------

// response429 handles too-many-request responses.
func (d *Download) response429(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	discardData(resp.Body) // the body is normally empty, but we discard anything present
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
