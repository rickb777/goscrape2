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
	type filePathCase struct {
		downloadURL      string
		expectedFilePath string
	}

	var cases = []filePathCase{
		{downloadURL: "https://github.com/#fragment", expectedFilePath: "./index.html"},
		{downloadURL: "https://github.com/.abc#fragment", expectedFilePath: "./.abc"},
		{downloadURL: "https://github.com/test#fragment", expectedFilePath: "./test.html"},
		{downloadURL: "https://github.com/test/#fragment", expectedFilePath: "./test/index.html"},
		{downloadURL: "https://github.com/test.aspx#fragment", expectedFilePath: "./test.aspx"},
		{downloadURL: "https://github.com/test/info.aspx#fragment", expectedFilePath: "./test/info.aspx"},
		{downloadURL: "https://github.com/test/info.aspx?a=1&b=2&b=3#fragment", expectedFilePath: "./test/info_a=1_b=2_b=3.aspx"},
		{downloadURL: "https://google.com/settings?year=2006&month=11#fragment", expectedFilePath: "./settings_month=11_year=2006.html"},
		{downloadURL: "https://google.com/settings/?year=2006&month=11#fragment", expectedFilePath: "./settings/month=11_year=2006.html"},
	}

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, c := range cases {
		url := must(c.downloadURL)

		output := GetFilePath(url, true)
		assert.Equal(t, c.expectedFilePath, output)
	}
}

func must(s string) *urlpkg.URL {
	u, e := urlpkg.Parse(s)
	if e != nil {
		panic(e)
	}
	return u
}
