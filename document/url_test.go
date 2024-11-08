package document

import (
	url "net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveURL(t *testing.T) {
	type filePathCase struct {
		baseURL        url.URL
		reference      string
		relativeToRoot string
		resolved       string
	}

	pathlessURL := url.URL{
		Scheme: "https",
		Host:   "petpic.xyz",
		Path:   "",
	}

	URL := url.URL{
		Scheme: "https",
		Host:   "petpic.xyz",
		Path:   "/earth/",
	}

	var cases = []filePathCase{
		{baseURL: pathlessURL, resolved: "index.html"},
		{baseURL: pathlessURL, reference: "#contents", resolved: "#contents"},
		{baseURL: pathlessURL, reference: "//any.other.xyz/a/path", resolved: "//any.other.xyz/a/path"},
		{baseURL: URL, reference: "brasil/index.html", resolved: "brasil/index.html"},
		{baseURL: URL, reference: "brasil/rio/index.html", resolved: "brasil/rio/index.html"},
		{baseURL: URL, reference: "../argentina/cat.jpg", resolved: "../argentina/cat.jpg"},
	}

	for _, c := range cases {
		resolved := resolveURL(&c.baseURL, c.reference, URL.Host, c.relativeToRoot)
		assert.Equal(t, c.resolved, resolved)
	}
}

func Test_urlRelativeToOther(t *testing.T) {
	type filePathCase struct {
		srcURL          url.URL
		baseURL         url.URL
		expectedSrcPath string
	}

	var cases = []filePathCase{
		{srcURL: url.URL{Path: "/earth/brasil/rio/cat.jpg"}, baseURL: url.URL{Path: "/earth/brasil/rio/"}, expectedSrcPath: "cat.jpg"},
		{srcURL: url.URL{Path: "/earth/brasil/rio/cat.jpg"}, baseURL: url.URL{Path: "/earth/"}, expectedSrcPath: "brasil/rio/cat.jpg"},
		{srcURL: url.URL{Path: "/earth/cat.jpg"}, baseURL: url.URL{Path: "/earth/brasil/rio/"}, expectedSrcPath: "../../cat.jpg"},
		{srcURL: url.URL{Path: "/earth/argentina/cat.jpg"}, baseURL: url.URL{Path: "/earth/brasil/rio/"}, expectedSrcPath: "../../argentina/cat.jpg"},
		{srcURL: url.URL{Path: "/earth/brasil/rio/cat.jpg"}, baseURL: url.URL{Path: "/mars/dogtown/"}, expectedSrcPath: "../../earth/brasil/rio/cat.jpg"},
		{srcURL: url.URL{Path: "///earth//////cat.jpg"}, baseURL: url.URL{Path: "///earth/brasil//rio////////"}, expectedSrcPath: "../../cat.jpg"},
	}

	for _, c := range cases {
		relativeURL := urlRelativeToOther(&c.srcURL, &c.baseURL)
		assert.Equal(t, c.expectedSrcPath, relativeURL)
	}
}

func Test_urlRelativeToRoot(t *testing.T) {
	type urlCases struct {
		srcURL   url.URL
		expected string
	}

	var cases = []urlCases{
		{srcURL: url.URL{Path: "/earth/brasil/rio/cat.jpg"}, expected: "../../../"},
		{srcURL: url.URL{Path: "cat.jpg"}},
		{srcURL: url.URL{Path: "/earth/argentina"}, expected: "../"},
		{srcURL: url.URL{Path: "///earth//////cat.jpg"}, expected: "../"},
	}

	for _, c := range cases {
		relativeURL := urlRelativeToRoot(&c.srcURL)
		assert.Equal(t, c.expected, relativeURL)
	}
}
