package download

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/cornelk/goscrape/document"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/work"
)

func (d *Download) response304(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	ext := strings.ToLower(path.Ext(item.URL.Path))

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
	return resp.Request.URL, &work.Result{Item: item, StatusCode: http.StatusNotModified}, nil
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
	return resp.Request.URL, &work.Result{Item: item, StatusCode: http.StatusNotModified, References: references}, nil
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

	return nil, &work.Result{Item: item, StatusCode: http.StatusNotModified, References: references}, nil
}
