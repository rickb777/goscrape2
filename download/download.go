package download

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/cornelk/gotokit/log"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"
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

	Throttle Throttle // increases when server gives 429 (Too Many Requests) responses
}

func (d *Download) ProcessURL(ctx context.Context, item work.Item) (*url.URL, htmlindex.Refs, error) {
	//logger.Info("Downloading", log.String("url", item.URL.String()))

	var references htmlindex.Refs

	resp, err := d.GET(ctx, item.URL)
	if err != nil {
		logger.Error("Processing HTTP Request failed",
			log.String("url", item.URL.String()),
			log.Err(err))
		return nil, nil, err
	}

	if resp == nil {
		return nil, nil, nil //response was 304-not modified
	}

	defer closeResponseBody(resp)

	if item.Depth == 0 {
		// take account of redirection (only on the start page)
		item.URL = resp.Request.URL
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		logger.Warn("Too many requests", log.String("url", item.URL.String()), log.Int("code", resp.StatusCode))
		// put this URL back into the work queue to be re-tried later
		return item.URL, htmlindex.Refs{item.URL}, nil

	case http.StatusNoContent:
		logger.Info("No content", log.String("url", item.URL.String()), log.Int("code", resp.StatusCode))
		// put this URL back into the work queue to be re-tried later
		return nil, nil, nil
	}

	contentType := header.ParseContentTypeFromHeaders(resp.Header)
	lastModified, _ := header.ParseHTTPDateTime(resp.Header.Get(headername.LastModified))

	isHtml := contentType.Type == "text" && contentType.Subtype == "html"
	isXHtml := contentType.Type == "application" && contentType.Subtype == "xhtml+xml"

	switch {
	case isHtml || isXHtml:
		data, err := bufferEntireResponse(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("buffering %s: %w", contentType.String(), err)
		}

		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("parsing %s: %w", contentType.String(), err)
		}

		index := htmlindex.New()
		index.Index(item.URL, doc)

		fixed, hasChanges, err := d.fixURLReferences(item.URL, doc, index)
		if err != nil {
			logger.Error("Fixing file references failed",
				log.String("url", item.URL.String()),
				log.Err(err))
			return nil, nil, nil
		}

		var rdr io.Reader
		if hasChanges {
			rdr = bytes.NewReader(fixed)
		} else {
			rdr = bytes.NewReader(data)
		}

		d.storeDownload(item.URL, rdr, lastModified, true)

		references, err = d.findReferences(index)
		if err != nil {
			return nil, nil, err
		}

	case contentType.Type == "text" && contentType.Subtype == "css":
		data, err := bufferEntireResponse(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("buffering text/scs: %w", err)
		}

		data, references = d.checkCSSForUrls(item.URL, data)

		d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	case contentType.Type == "image" && d.Config.ImageQuality != 0:
		data, err := bufferEntireResponse(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("buffering %s: %w", contentType.String(), err)
		}

		data = d.Config.ImageQuality.CheckImageForRecode(item.URL, data)
		if d.Config.ImageQuality != 0 {
			lastModified = time.Time{} // altered images can't be safely time-stamped
		}

		d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	default:
		d.storeDownload(item.URL, resp.Body, lastModified, false)
	}

	logger.Info("OK", log.String("url", item.URL.String()), log.Int("code", resp.StatusCode))

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, references, nil
}

//-------------------------------------------------------------------------------------------------

var tagsWithReferences = []string{
	htmlindex.ATag,
	htmlindex.LinkTag,
	htmlindex.ScriptTag,
	htmlindex.BodyTag,
}

func (d *Download) findReferences(index *htmlindex.Index) (htmlindex.Refs, error) {
	var result htmlindex.Refs
	for _, tag := range tagsWithReferences {
		references, err := index.URLs(tag)
		if err != nil {
			logger.Error("Getting node URLs failed",
				log.String("node", tag),
				log.Err(err))
		}

		for _, ur := range references {
			ur.Fragment = ""
			result = append(result, ur)
		}
	}

	references, err := index.URLs(htmlindex.ImgTag)
	if err != nil {
		logger.Error("Getting img node URLs failed", log.Err(err))
	}

	for _, ur := range references {
		ur.Fragment = ""
		result = append(result, ur)
	}

	return result, nil
}

// storeDownload writes the download to a file, if a known binary file is detected,
// processing of the file as page to look for links is skipped.
func (d *Download) storeDownload(u *url.URL, data io.Reader, lastModified time.Time, isAPage bool) {
	filePath := d.getFilePath(u, isAPage)

	if !isAPage && FileExists(filePath) {
		return
	}

	if err := WriteFile(d.StartURL, filePath, data); err != nil {
		logger.Error("Writing to file failed",
			log.String("URL", u.String()),
			log.String("file", filePath),
			log.Err(err))
		return
	}

	if !lastModified.IsZero() {
		if err := os.Chtimes(filePath, lastModified, lastModified); err != nil {
			logger.Error("Updating file timestamps failed",
				log.String("URL", u.String()),
				log.String("file", filePath),
				log.Err(err))
		}
	}
}
