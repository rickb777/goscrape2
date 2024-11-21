package document

import (
	"io"
	"log/slog"
	"net/url"
	"os"
	"testing"

	"github.com/cornelk/goscrape/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFilePath(t *testing.T) {
	type filePathCase struct {
		baseURL          string
		downloadURL      string
		expectedFilePath string
	}

	pathSeparator := string(os.PathSeparator)
	expectedBasePath := "x/google.com" + pathSeparator

	var cases = []filePathCase{
		{baseURL: "https://google.com/", downloadURL: "https://github.com/", expectedFilePath: expectedBasePath + "_github.com" + pathSeparator + "index.html"},
		{baseURL: "https://google.com/", downloadURL: "https://github.com/#fragment", expectedFilePath: expectedBasePath + "_github.com" + pathSeparator + "index.html"},
		{baseURL: "https://google.com/", downloadURL: "https://github.com/test", expectedFilePath: expectedBasePath + "_github.com" + pathSeparator + "test.html"},
		{baseURL: "https://google.com/", downloadURL: "https://github.com/test/", expectedFilePath: expectedBasePath + "_github.com" + pathSeparator + "test" + pathSeparator + "index.html"},
		{baseURL: "https://google.com/", downloadURL: "https://github.com/test.aspx", expectedFilePath: expectedBasePath + "_github.com" + pathSeparator + "test.aspx"},
		{baseURL: "https://google.com/", downloadURL: "https://google.com/settings", expectedFilePath: expectedBasePath + "settings.html"},
	}

	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, c := range cases {
		URL, err := url.Parse(c.downloadURL)
		require.NoError(t, err)

		output := GetFilePath(URL, must(c.baseURL), "x", true)
		assert.Equal(t, c.expectedFilePath, output)
	}
}

func must(s string) *url.URL {
	u, e := url.Parse(s)
	if e != nil {
		panic(e)
	}
	return u
}
