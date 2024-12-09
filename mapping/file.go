package mapping

import (
	"github.com/rickb777/path"
	"net/url"
	"strings"
)

const (
	// HTMLExtension is the file extension that downloaded pages get.
	HTMLExtension = ".html"

	// PageDirIndex is the file name of the index file for every dir.
	PageDirIndex = "index" + HTMLExtension
)

// GetFilePath returns a file path for a URL to store the URL content in.
func GetFilePath(url *url.URL, isAPage bool) path.Path {
	tailingSlash := strings.HasSuffix(url.Path, "/")
	if isAPage || tailingSlash {
		fileName := GetPageFilePath(url)
		return "." + fileName
	} else {
		return path.Path("." + url.Path)
	}
}

// GetPageFilePath returns a filename for a URL that represents a page.
func GetPageFilePath(url *url.URL) path.Path {
	fileName := url.Path

	switch {
	case fileName == "":
		fileName = "/" + PageDirIndex
		// directory index will be index.html in the directory

	case fileName[len(fileName)-1] == '/':
		fileName += PageDirIndex

	default:
		ext := path.Ext(fileName)
		// if file extension is missing add .html, otherwise keep the existing file extension
		if ext == "" {
			fileName += HTMLExtension
		}
	}

	return path.Path(fileName)
}
