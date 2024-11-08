package document

import (
	"net/url"
	"path/filepath"
)

const externalDomainPrefix = "_" // _ is a prefix for external domains on the filesystem

// GetFilePath returns a file path for a URL to store the URL content in.
func GetFilePath(url, startURL *url.URL, outputDirectory string, isAPage bool) string {
	fileName := url.Path
	if isAPage {
		fileName = GetPageFilePath(url)
	}

	var externalHost string
	if url.Host != startURL.Host {
		externalHost = externalDomainPrefix + url.Host
	}

	return filepath.Join(outputDirectory, startURL.Host, externalHost, fileName)
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
