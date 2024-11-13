package download

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/document"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/acceptable/headername"
)

func (d *Download) response200(item work.Item, resp *http.Response) (*url.URL, *work.Result, error) {
	contentType := header.ParseContentTypeFromHeaders(resp.Header)
	lastModified, _ := header.ParseHTTPDateTime(resp.Header.Get(headername.LastModified))
	isGzip := resp.Header.Get(headername.ContentEncoding) == "gzip"

	metadata := db.Item{ETags: resp.Header.Get(headername.ETag)}
	if expires := resp.Header.Get(headername.Expires); expires != "" {
		metadata.Expires, _ = header.ParseHTTPDateTime(expires)
		metadata.Expires = metadata.Expires
	}

	d.ETagsDB.Store(item.URL, metadata)

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

	contentLength := int64(len(data))

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
	fileSize := d.storeDownload(item.URL, rdr, lastModified, true)

	references, err = doc.FindReferences()
	if err != nil {
		return nil, nil, err
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, &work.Result{Item: item, StatusCode: resp.StatusCode, ContentLength: contentLength, FileSize: fileSize, Gzip: isGzip, References: references}, nil
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
//	return resp.Request.URL, &work.Result{Item: item, StatusCode: resp.StatusCode, ContentLength: contentLength, FileSize: fileSize, References: references}, nil
//}

//-------------------------------------------------------------------------------------------------

func (d *Download) css200(item work.Item, resp *http.Response, lastModified time.Time, isGzip bool) (*url.URL, *work.Result, error) {
	var references work.Refs

	data, err := bufferEntireResponse(resp, isGzip)
	if err != nil {
		return nil, nil, fmt.Errorf("buffering text/css: %w", err)
	}

	contentLength := int64(len(data))

	data, references = document.CheckCSSForUrls(item.URL, d.StartURL.Host, data)

	fileSize := d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	return nil, &work.Result{Item: item, StatusCode: resp.StatusCode, ContentLength: contentLength, FileSize: fileSize, Gzip: isGzip, References: references}, nil
}

//-------------------------------------------------------------------------------------------------

func (d *Download) image200(item work.Item, resp *http.Response, lastModified time.Time, contentType header.ContentType, isGzip bool) (*url.URL, *work.Result, error) {
	data, err := bufferEntireResponse(resp, isGzip)
	if err != nil {
		return nil, nil, fmt.Errorf("buffering %s: %w", contentType.String(), err)
	}

	contentLength := int64(len(data))

	data = d.Config.ImageQuality.CheckImageForRecode(item.URL, data)
	if d.Config.ImageQuality != 0 {
		lastModified = time.Time{} // altered images can't be safely time-stamped
	}

	fileSize := d.storeDownload(item.URL, bytes.NewReader(data), lastModified, false)

	return nil, &work.Result{Item: item, StatusCode: resp.StatusCode, ContentLength: contentLength, Gzip: isGzip, FileSize: fileSize}, nil
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
	fileSize := d.storeDownload(item.URL, rdr, lastModified, false)

	return nil, &work.Result{Item: item, StatusCode: resp.StatusCode, FileSize: fileSize, Gzip: isGzip}, nil
}

//-------------------------------------------------------------------------------------------------

// storeDownload writes the download to a file, if a known binary file is detected,
// processing of the file as page to look for links is skipped.
func (d *Download) storeDownload(u *url.URL, data io.Reader, lastModified time.Time, isAPage bool) (fileSize int64) {
	filePath := document.GetFilePath(u, d.StartURL, d.Config.OutputDirectory, isAPage)

	if !isAPage && ioutil.FileExists(d.Fs, filePath) {
		return 0
	}

	var err error
	if fileSize, err = ioutil.WriteFileAtomically(d.Fs, d.StartURL, filePath, data); err != nil {
		logger.Error("Writing to file failed",
			slog.String("URL", u.String()),
			slog.String("file", filePath),
			slog.Any("error", err))
		return fileSize
	}

	if !lastModified.IsZero() {
		if err := d.Fs.Chtimes(filePath, lastModified, lastModified); err != nil {
			logger.Error("Updating file timestamps failed",
				slog.String("URL", u.String()),
				slog.String("file", filePath),
				slog.Any("error", err))
		}
	}

	return fileSize
}

//-------------------------------------------------------------------------------------------------

func bufferEntireResponse(resp *http.Response, isGzip bool) ([]byte, error) {
	buf := &bytes.Buffer{}
	var err error

	rdr := resp.Body
	if isGzip {
		rdr, err = gzip.NewReader(rdr)
		if err != nil {
			logger.Error("Decompressing gzip response failed",
				slog.Any("url", resp.Request.URL),
				slog.Any("error", err))
			return nil, err
		}
		defer rdr.Close() // this only closes the gzipper, not the response body
	}

	if _, err := io.Copy(buf, rdr); err != nil {
		return nil, fmt.Errorf("%s reading response body: %w", resp.Request.URL, err)
	}
	return buf.Bytes(), nil
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
