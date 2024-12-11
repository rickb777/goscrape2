package mapping

import (
	"fmt"
	"github.com/rickb777/path"
	"net/url"
	"slices"
	"strings"
)

const (
	// htmlExtension is the file extension that downloaded pages get.
	htmlExtension = ".html"

	// pageDirIndex is the file name of the index file for every dir.
	pageDirIndex = "index"
)

// GetFilePath returns a file path for a URL to store the URL content in.
func GetFilePath(url *url.URL, isAPage bool) string {
	trailingSlash := strings.HasSuffix(url.Path, "/")
	if trailingSlash {
		return "." + getHTMLPathWithTrailingSlash(url)
	} else if isAPage {
		return "." + getHTMLPathWithName(url)
	} else {
		name, ext := path.SplitExt(url.Path)
		return "." + name + prefixNonBlank(fileSafeQueryString(url.Query())) + ext
	}
}

// GetPageFilePath returns a filename for a URL that represents a page.
func GetPageFilePath(url *url.URL) string {
	trailingSlash := strings.HasSuffix(url.Path, "/")
	if trailingSlash {
		return getHTMLPathWithTrailingSlash(url)
	} else {
		return getHTMLPathWithName(url)
	}
}

func getHTMLPathWithTrailingSlash(url *url.URL) string {
	fileName := url.Path
	qs := fileSafeQueryString(url.Query())

	switch {
	case fileName == "":
		fileName = "/" + pageDirIndex + prefixNonBlank(qs) + htmlExtension

	case qs == "":
		fileName += pageDirIndex + htmlExtension

	default:
		fileName += qs + htmlExtension
	}

	return fileName
}

func getHTMLPathWithName(url *url.URL) string {
	fileName := url.Path
	qs := fileSafeQueryString(url.Query())

	switch {
	case fileName == "":
		fileName = "/" + pageDirIndex + prefixNonBlank(qs) + htmlExtension
		// directory index will be index.html in the directory

	default:
		name, ext := path.SplitExt(fileName)
		// if file extension is missing add .html, otherwise keep the existing file extension
		if ext == "" {
			ext = htmlExtension
		}
		if name != "" {
			fileName = name + prefixNonBlank(qs) + ext
		} else {
			fileName = qs + ext
		}
	}

	return fileName
}

func fileSafeQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	nSegments := 0

	var keys []string
	keys = make([]string, 0, len(values))
	for k, vs := range values {
		keys = append(keys, k)
		nSegments += len(vs)
	}
	slices.Sort(keys)

	segments := make([]string, 0, nSegments)

	for _, k := range keys {
		vs := values[k]
		for _, v := range vs {
			segments = append(segments, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return strings.Join(segments, "_")
}

func prefixNonBlank(s string) string {
	if s == "" {
		return s
	}
	return "_" + s
}
