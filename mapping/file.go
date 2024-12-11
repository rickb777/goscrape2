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
	if url.Path == "" {
		return "." + defaultName(url.Query())
	}

	switch {
	case strings.HasSuffix(url.Path, "/"):
		return "." + urlEndsWithSlash(url)

	case isAPage:
		return "." + urlEndsWithName(url)

	default:
		name, ext := path.SplitExt(url.Path)
		return "." + name + prefixNonBlank(fileSafeQueryString(url.Query())) + ext
	}
}

// GetPageFilePath returns a filename for a URL that represents a page.
func GetPageFilePath(url *url.URL) string {
	if url.Path == "" {
		return defaultName(url.Query())
	}

	if strings.HasSuffix(url.Path, "/") {
		return urlEndsWithSlash(url)
	} else {
		return urlEndsWithName(url)
	}
}

func urlEndsWithSlash(url *url.URL) string {
	query := url.Query()

	if len(query) == 0 {
		return url.Path + pageDirIndex + htmlExtension
	}

	qs := fileSafeQueryString(query)

	return url.Path + qs + htmlExtension
}

func urlEndsWithName(url *url.URL) string {
	qs := fileSafeQueryString(url.Query())

	name, ext := path.SplitExt(url.Path)
	if ext == "" {
		ext = htmlExtension
	}

	return name + prefixNonBlank(qs) + ext
}

func defaultName(qs url.Values) string {
	return "/___" + fileSafeQueryString(qs) + htmlExtension
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
