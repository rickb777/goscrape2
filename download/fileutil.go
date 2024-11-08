package download

import (
	"github.com/cornelk/goscrape/document"
	"net/url"
	"path/filepath"
)

const (
	// PageExtension is the file extension that downloaded pages get.
	PageExtension = document.PageExtension
	// PageDirIndex is the file name of the index file for every dir.
	PageDirIndex = document.PageDirIndex
)

const externalDomainPrefix = "_" // _ is a prefix for external domains on the filesystem

// getFilePath returns a file path for a URL to store the URL content in.
func (d *Download) getFilePath(url *url.URL, isAPage bool) string {
	fileName := url.Path
	if isAPage {
		fileName = document.GetPageFilePath(url)
	}

	var externalHost string
	if url.Host != d.StartURL.Host {
		externalHost = externalDomainPrefix + url.Host
	}

	return filepath.Join(d.Config.OutputDirectory, d.StartURL.Host, externalHost, fileName)
}
