package download

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/document"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"github.com/spf13/afero"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Download fetches URLs one by one, sequentially.
type Download struct {
	Config   config.Config
	Cookies  *cookiejar.Jar
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

	startTime := time.Now()

	resp, err := d.GET(ctx, item.URL, existingModified)
	if err != nil {
		logger.Error("Processing HTTP Request failed",
			slog.String("url", item.URL.String()),
			slog.Any("error", err))
		return nil, nil, err
	}

	if resp == nil {
		panic("unexpected nil response")
	}

	defer closeResponseBody(resp.Body, resp.Request.URL)
	defer logResponse(item.URL, resp, startTime)

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
		// put this URL back into the work queue to be re-tried later
		repeat := &work.Result{Item: item, References: []*url.URL{item.URL}}
		repeat.Item.Depth-- // because it will get incremented
		return item.URL, repeat, nil

	default:
		noFurtherAction := &work.Result{Item: item}
		return item.URL, noFurtherAction, nil
	}
}

//-------------------------------------------------------------------------------------------------

func (d *Download) response200(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	contentType := header.ParseContentTypeFromHeaders(resp.Header)
	lastModified, _ := header.ParseHTTPDateTime(resp.Header.Get(headername.LastModified))
	isGzip := resp.Header.Get(headername.ContentEncoding) == "gzip"

	switch {
	case isHtml(contentType) || isXHtml(contentType):
		return d.html200(item, resp, lastModified, contentType, isGzip)

	case isCSS(contentType):
		return d.css200(item, resp, lastModified, isGzip)

	//case isSVG(contentType):
	//	return d.svg200(item, resp, lastModified, isGzip)

	case contentType.Type == "image" && d.Config.ImageQuality != 0:
		return d.image200(item, resp, lastModified, contentType, isGzip)

	default:
		return d.other200(item, resp, lastModified, isGzip)
	}
}

//-------------------------------------------------------------------------------------------------

func (d *Download) html200(item work.Item, resp *http.Response, lastModified time.Time, contentType header.ContentType, isGzip bool) (*url.URL, *work.Result, error) {
	var references work.Refs

	data, err := bufferEntireResponse(resp, isGzip)
	if err != nil {
		return nil, nil, fmt.Errorf("buffering %s: %w", contentType.String(), err)
	}

	doc, err := document.ParseHTML(item.URL, d.StartURL, bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", contentType.String(), err)
	}

	fixed, hasChanges, err := doc.FixURLReferences()
	if err != nil {
		logger.Error("Fixing file references failed",
			slog.String("url", item.String()),
			slog.Any("error", err))
		return nil, nil, nil
	}

	if hasChanges {
		data = fixed
	}
	rdr := bytes.NewReader(data)
	d.storeDownload(item.URL, rdr, lastModified, true)

	references, err = doc.FindReferences()
	if err != nil {
		return nil, nil, err
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, &work.Result{Item: item, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

//func (d *Download) svg200(item work.Item, resp *http.Response, lastModified time.Time, isGzip bool) (*url.URL, *work.Result, error) {
//	var references work.Refs
//
//	data, err := bufferEntireResponse(resp, isGzip)
//	if err != nil {
//		return nil, nil, fmt.Errorf("buffering SVG: %w", err)
//	}
//
//	doc, err := document.ParseSVG(item.URL, d.StartURL, bytes.NewReader(data))
//	if err != nil {
//		return nil, nil, fmt.Errorf("SVG: %w", err)
//	}
//
//	fixed, hasChanges, err := doc.FixURLReferences()
//	if err != nil {
//		logger.Error("Fixing file references failed",
//			slog.String("url", item.String()),
//			slog.Any("error", err))
//		return nil, nil, nil
//	}
//
//	if hasChanges {
//		data = fixed
//	}
//	rdr := bytes.NewReader(data)
//	d.storeDownload(item.URL, rdr, lastModified, true)
//
//	references, err = doc.FindReferences()
//	if err != nil {
//		return nil, nil, err
//	}
//
//	// use the URL that the website returned as new base url for the
//	// scrape, in case a redirect changed it (only for the start page)
//	return resp.Request.URL, &work.Result{Item: item, References: references}, nil
//}

//-------------------------------------------------------------------------------------------------

func (d *Download) css200(item work.Item, resp *http.Response, lastModified time.Time, isGzip bool) (*url.URL, *work.Result, error) {
	var references work.Refs

	data, err := bufferEntireResponse(resp, isGzip)
	if err != nil {
		return nil, nil, fmt.Errorf("buffering text/css: %w", err)
	}

	data, references = document.CheckCSSForUrls(item.URL, d.StartURL.Host, data)

	d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	return nil, &work.Result{Item: item, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) image200(item work.Item, resp *http.Response, lastModified time.Time, contentType header.ContentType, isGzip bool) (*url.URL, *work.Result, error) {
	data, err := bufferEntireResponse(resp, isGzip)
	if err != nil {
		return nil, nil, fmt.Errorf("buffering %s: %w", contentType.String(), err)
	}

	data = d.Config.ImageQuality.CheckImageForRecode(item.URL, data)
	if d.Config.ImageQuality != 0 {
		lastModified = time.Time{} // altered images can't be safely time-stamped
	}

	d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	return nil, &work.Result{Item: item}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) other200(item work.Item, resp *http.Response, lastModified time.Time, isGzip bool) (*url.URL, *work.Result, error) {
	rdr := resp.Body
	if isGzip {
		var err error
		rdr, err = gzip.NewReader(rdr)
		if err != nil {
			logger.Error("Decompressing gzip response failed",
				slog.Any("url", resp.Request.URL),
				slog.Any("error", err))
			return nil, nil, err
		}
		defer rdr.Close() // this only closes the gzipper, not the response body
	}

	// store without buffering entire file into memory
	d.storeDownload(item.URL, rdr, lastModified, false)

	return nil, &work.Result{Item: item}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) response304(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	ext := path.Ext(item.URL.Path)

	switch ext {
	case ".html", ".htm":
		return d.html304(item, resp)

	case ".css":
		return d.css304(item)

	default:
		if strings.HasSuffix(item.URL.Path, "/") {
			return d.html304(item, resp)
		}
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, &work.Result{Item: item}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) html304(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	var references work.Refs

	filePath := document.GetFilePath(item.URL, d.StartURL, d.Config.OutputDirectory, true)
	data, err := ioutil.ReadFile(d.Fs, d.StartURL, filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("existing HTML file: %w", err)
	}

	doc, err := document.ParseHTML(item.URL, d.StartURL, bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("parsing HTML: %w", err)
	}

	references, err = doc.FindReferences()
	if err != nil {
		return nil, nil, err
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, &work.Result{Item: item, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) css304(item work.Item) (*url.URL, *work.Result, error) {
	var references work.Refs
	filePath := document.GetFilePath(item.URL, d.StartURL, d.Config.OutputDirectory, false)
	data, err := ioutil.ReadFile(d.Fs, d.StartURL, filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("existing CSS file: %w", err)
	}

	_, references = document.CheckCSSForUrls(item.URL, d.StartURL.Host, data)

	return nil, &work.Result{Item: item, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

// storeDownload writes the download to a file, if a known binary file is detected,
// processing of the file as page to look for links is skipped.
func (d *Download) storeDownload(u *url.URL, data io.Reader, lastModified time.Time, isAPage bool) {
	filePath := document.GetFilePath(u, d.StartURL, d.Config.OutputDirectory, isAPage)

	if !isAPage && ioutil.FileExists(d.Fs, filePath) {
		return
	}

	if err := ioutil.WriteFileAtomically(d.Fs, d.StartURL, filePath, data); err != nil {
		logger.Error("Writing to file failed",
			slog.String("URL", u.String()),
			slog.String("file", filePath),
			slog.Any("error", err))
		return
	}

	if !lastModified.IsZero() {
		if err := os.Chtimes(filePath, lastModified, lastModified); err != nil {
			logger.Error("Updating file timestamps failed",
				slog.String("URL", u.String()),
				slog.String("file", filePath),
				slog.Any("error", err))
		}
	}
}

//-------------------------------------------------------------------------------------------------

func isHtml(contentType header.ContentType) bool {
	return contentType.Type == "text" && contentType.Subtype == "html"
}

func isXHtml(contentType header.ContentType) bool {
	return contentType.Type == "application" && contentType.Subtype == "xhtml+xml"
}

func isCSS(contentType header.ContentType) bool {
	return contentType.Type == "text" && contentType.Subtype == "css"
}

func isSVG(contentType header.ContentType) bool {
	return contentType.Type == "image" && contentType.Subtype == "svg+xml"
}

func timeTaken(before time.Time) string {
	return time.Now().Sub(before).Round(time.Millisecond).String()
}

func logResponse(item *url.URL, resp *http.Response, before time.Time) {
	level := slog.LevelInfo
	switch {
	case resp.StatusCode >= 400:
		level = slog.LevelWarn
	}
	logger.Log(level, http.StatusText(resp.StatusCode), slog.String("url", item.String()),
		slog.Int("code", resp.StatusCode), slog.String("took", timeTaken(before)))
}
