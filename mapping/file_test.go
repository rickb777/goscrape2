package mapping

import (
	"io"
	"log/slog"
	urlpkg "net/url"
	"testing"

	"github.com/rickb777/goscrape2/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetFilePath(t *testing.T) {
	must := func(url string) *urlpkg.URL {
		return mustParse(t, url)
	}

	var cases = []struct {
		isAPage     bool
		downloadURL *urlpkg.URL
		expected    string
	}{
		{isAPage: true, downloadURL: must("https://github.com/#fragment"), expected: "./index.html"},
		{isAPage: true, downloadURL: must("https://github.com/test#fragment"), expected: "./test.html"},
		{isAPage: false, downloadURL: must("https://github.com/test/#fragment"), expected: "./test/index.html"},
		{isAPage: false, downloadURL: must("https://github.com/test/page+info.aspx#fragment"), expected: "./test/page+info.aspx"},
		{isAPage: false, downloadURL: must("https://github.com/test/page+info.aspx?a=1&b=2&b=3#fragment"), expected: "./test/page+info_a=1_b=2_b=3.aspx"},
		{isAPage: false, downloadURL: must("https://github.com/test/page+info.aspx?a=1&b=2&b=%33#fragment"), expected: "./test/page+info_a=1_b=2_b=3.aspx"},
		{isAPage: false, downloadURL: must("https://github.com/?a=%31&b=2&b=3#fragment"), expected: "./a=1_b=2_b=3.html"},
		// edge cases
		{downloadURL: &urlpkg.URL{}, expected: "./___.html"},
		{downloadURL: &urlpkg.URL{RawQuery: "a=1&b=4&b=3"}, expected: "./___a=1_b=4_b=3.html"},
	}

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, c := range cases {
		output := GetFilePath(c.downloadURL, c.isAPage)
		assert.Equal(t, c.expected, output)
	}
}

func TestGetPageFilePath(t *testing.T) {
	must := func(url string) *urlpkg.URL {
		return mustParse(t, url)
	}

	var cases = []struct {
		downloadURL *urlpkg.URL
		expected    string
	}{
		{downloadURL: must("https://github.com/#fragment"), expected: "/index.html"},
		{downloadURL: must("https://github.com/.abc#fragment"), expected: "/.abc"},
		{downloadURL: must("https://github.com/test#fragment"), expected: "/test.html"},
		{downloadURL: must("https://github.com/test/#fragment"), expected: "/test/index.html"},
		{downloadURL: must("https://github.com/test.aspx#fragment"), expected: "/test.aspx"},
		{downloadURL: must("https://github.com/test/page+info.aspx#fragment"), expected: "/test/page+info.aspx"},
		{downloadURL: must("https://github.com/test/page%2Binfo.aspx?a=1&b=2&b=3#fragment"), expected: "/test/page+info_a=1_b=2_b=3.aspx"},
		{downloadURL: must("https://google.com/settings?year=2006&month=11#fragment"), expected: "/settings_month=11_year=2006.html"},
		{downloadURL: must("https://google.com/settings/?year=2006&month=11#fragment"), expected: "/settings/month=11_year=2006.html"},
		// edge cases
		{downloadURL: &urlpkg.URL{}, expected: "/___.html"},
		{downloadURL: &urlpkg.URL{RawQuery: "a=1&b=4&b=3"}, expected: "/___a=1_b=4_b=3.html"},
	}

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, c := range cases {
		output := GetPageFilePath(c.downloadURL)
		assert.Equal(t, c.expected, output)
	}
}

func mustParse(t *testing.T, url string) *urlpkg.URL {
	u, err := urlpkg.Parse(url)
	if err != nil {
		t.Fatalf("Parse(%q) got err %v", url, err)
	}
	return u
}
