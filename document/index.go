package document

import (
	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"log/slog"
	"net/url"
	"path/filepath"
)

const (
	// PageExtension is the file extension that downloaded pages get.
	PageExtension = ".html"
	// PageDirIndex is the file name of the index file for every dir.
	PageDirIndex = "index" + PageExtension
)

func (d *Document) FindReferences() (work.Refs, error) {
	var result work.Refs
	for tag := range htmlindex.Nodes {
		references, err := d.index.URLs(tag)
		if err != nil {
			logger.Error("Getting node URLs failed",
				slog.String("url", d.u.String()),
				slog.String("node", tag.String()),
				slog.Any("error", err))
		}

		for _, ur := range references {
			ur.Fragment = ""
			result = append(result, ur)
		}
	}

	//references, err := d.index.URLs(atom.Img)
	//if err != nil {
	//	logger.Error("Getting <img> URLs failed", slog.String("url", d.u.String()), slog.Any("error", err))
	//}
	//
	//for _, ur := range references {
	//	ur.Fragment = ""
	//	result = append(result, ur)
	//}

	return result, nil
}

// GetPageFilePath returns a filename for a URL that represents a page.
func GetPageFilePath(url *url.URL) string {
	fileName := url.Path

	// root of domain will be index.html
	switch {
	case fileName == "" || fileName == "/":
		fileName = PageDirIndex
		// directory index will be index.html in the directory

	case fileName[len(fileName)-1] == '/':
		fileName += PageDirIndex

	default:
		ext := filepath.Ext(fileName)
		// if file extension is missing add .html, otherwise keep the existing file extension
		if ext == "" {
			fileName += PageExtension
		}
	}

	return fileName
}
