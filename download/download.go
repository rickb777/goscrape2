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
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

// assetProcessor is a processor of a downloaded asset that can transform
// a downloaded file content before it will be stored on disk.
type assetProcessor func(URL *url.URL, data []byte) []byte

var tagsWithReferences = []string{
	htmlindex.ATag,
	htmlindex.LinkTag,
	htmlindex.ScriptTag,
	htmlindex.BodyTag,
}

type Download struct {
	Config   config.Config
	Cookies  *cookiejar.Jar
	StartURL *url.URL

	Auth   string
	Client *http.Client
}

func (d *Download) ProcessURL(ctx context.Context, item work.Item) (*url.URL, []*url.URL, error) {
	logger.Info("Downloading", log.String("url", item.URL.String()))

	var references []*url.URL

	resp, err := DownloadURL(ctx, d, item.URL)
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

	//fileExtension := ""
	//kind, err := filetype.Match(data)
	//if err == nil && kind != types.Unknown {
	//	fileExtension = kind.Extension
	//}

	if item.Depth == 0 {
		// take account of redirection (only on the start page)
		item.URL = resp.Request.URL
	}

	contentType := header.ParseContentTypeFromHeaders(resp.Header)

	isHtml := contentType.Type == "text" && contentType.Subtype == "html"
	isXHtml := contentType.Type == "application" && contentType.Subtype == "xhtml+xml"

	switch {
	case isHtml || isXHtml:
		data, err := bufferEntireResponse(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("buffering HTML: %w", err)
		}

		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("parsing HTML: %w", err)
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

		d.storeDownload(item.URL, rdr, true)

		references, err = d.findReferences(index)
		if err != nil {
			return nil, nil, err
		}

	case contentType.Type == "text" && contentType.Subtype == "css":
		data, err := bufferEntireResponse(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("buffering HTML: %w", err)
		}

		data, references = d.checkCSSForUrls(item.URL, data)

		d.storeDownload(item.URL, bytes.NewReader(data), false)

	default:
		d.storeDownload(item.URL, resp.Body, false)
	}

	// use the URL that the website returned as new base url for the
	// scrape, in case a redirect changed it (only for the start page)
	return resp.Request.URL, references, nil
}

func (d *Download) findReferences(index *htmlindex.Index) ([]*url.URL, error) {
	var result []*url.URL
	for _, tag := range tagsWithReferences {
		references, err := index.URLs(tag)
		if err != nil {
			logger.Error("Getting node URLs failed",
				log.String("node", tag),
				log.Err(err))
		}

		//var processor assetProcessor
		//if tag == htmlindex.LinkTag {
		//	processor = s.checkCSSForUrls
		//}
		for _, ur := range references {
			ur.Fragment = ""
			result = append(result, ur)
			//err := s.downloadAsset(ctx, ur, processor)
			//if err != nil && errors.Is(err, context.Canceled) {
			//	return err
			//}
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

	//for _, image := range s.imagesQueue {
	//	if err := s.downloadAsset(ctx, image, s.checkImageForRecode); err != nil && errors.Is(err, context.Canceled) {
	//		return nil, err
	//	}
	//}
	return result, nil
}

// downloadAsset downloads an asset if it does not exist on disk yet.
//func (s *Download) downloadAsset(ctx context.Context, u *url.URL, processor assetProcessor) error {
//	u.Fragment = ""
//	urlFull := u.String()
//
//	//if !s.shouldURLBeDownloaded(work.Item{URL: u}, true) {
//	//	return nil
//	//}
//
//	filePath := s.getFilePath(u, false)
//	if FileExists(filePath) {
//		return nil
//	}
//
//	logger.Info("Downloading asset", log.String("url", urlFull))
//	resp, err := s.HttpDownloader(ctx, u)
//	if err != nil {
//		logger.Error("Downloading asset failed",
//			log.String("url", urlFull),
//			log.Err(err))
//		return fmt.Errorf("downloading asset: %w", err)
//	}
//
//	if resp == nil {
//		return nil // 304-not modified
//	}
//
//	defer closeResponseBody(resp)
//
//	var rdr io.Reader = resp.Body
//
//	if processor != nil {
//		data, err := bufferEntireResponse(resp)
//		if err != nil {
//			return fmt.Errorf("%s: downloading asset: %w", u, err)
//		}
//		rdr = bytes.NewReader(processor(u, data))
//	}
//
//	if err = WriteFile(s.StartURL, filePath, rdr); err != nil {
//		logger.Error("Writing asset file failed",
//			log.String("url", urlFull),
//			log.String("file", filePath),
//			log.Err(err))
//	}
//
//	return nil
//}

// storeDownload writes the download to a file, if a known binary file is detected,
// processing of the file as page to look for links is skipped.
func (d *Download) storeDownload(u *url.URL, data io.Reader, isAPage bool) {
	filePath := d.getFilePath(u, isAPage)

	if err := WriteFile(d.StartURL, filePath, data); err != nil {
		logger.Error("Writing to file failed",
			log.String("URL", u.String()),
			log.String("file", filePath),
			log.Err(err))
	}
}
