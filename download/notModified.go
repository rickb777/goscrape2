package download

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/rickb777/goscrape2/document"
	"github.com/rickb777/goscrape2/download/ioutil"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/mapping"
	"github.com/rickb777/goscrape2/work"
)

func (d *Download) response304(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	ext := strings.ToLower(path.Ext(item.URL.Path))

	switch ext {
	case ".html", ".htm":
		return d.html304(item, resp)

	case ".css":
		return d.css304(item, resp.StatusCode)

	default:
		if strings.HasSuffix(item.URL.Path, "/") {
			return d.html304(item, resp)
		}
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil
}

//-------------------------------------------------------------------------------------------------

// html304 reads the HTML file from disk so that all the URLs it references can be scraped
func (d *Download) html304(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	var references work.Refs

	filePath := mapping.GetFilePath(item.URL, true)
	data, err := ioutil.ReadFile(d.Fs, filePath)
	if err != nil {
		logger.Debug("absent HTML file", slog.Any("error", err))
		return nil, &work.Result{Item: item, StatusCode: resp.StatusCode}, nil
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
	return resp.Request.URL, &work.Result{Item: item, StatusCode: resp.StatusCode, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

// css304 reads the CSS file from disk so that all the URLs it references can be scraped
func (d *Download) css304(item work.Item, statusCode int) (*url.URL, *work.Result, error) {
	var references work.Refs
	filePath := mapping.GetFilePath(item.URL, false)
	data, err := ioutil.ReadFile(d.Fs, filePath)
	if err != nil {
		logger.Debug("absent CSS file", slog.Any("error", err))
		return nil, &work.Result{Item: item, StatusCode: statusCode}, nil
	}

	_, references = document.CheckCSSForUrls(item.URL, d.StartURL.Host, data)

	return nil, &work.Result{Item: item, StatusCode: statusCode, References: references}, nil
}
