package download

import (
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/logger"
	"github.com/stretchr/testify/assert"
)

func TestCheckCSSForURLs(t *testing.T) {
	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		URL: "http://localhost",
	}
	u, _ := url.Parse(cfg.URL)
	d := Download{Config: cfg, StartURL: u}

	var fixtures = map[string]string{
		"url('http://localhost/uri/between/single/quote')": "http://localhost/uri/between/single/quote",
		`url("http://localhost/uri/between/double/quote")`: "http://localhost/uri/between/double/quote",
		"url(http://localhost/uri)":                        "http://localhost/uri",
		"url(data:image/gif;base64,R0lGODl)":               "",
		`div#gopher {
			background: url(/doc/gopher/frontpage.png) no-repeat;
			height: 155px;
			}`: "http://localhost/doc/gopher/frontpage.png",
	}

	u, _ = url.Parse("http://localhost")
	for input, expected := range fixtures {
		_, refs := d.checkCSSForUrls(u, []byte(input))

		if expected == "" {
			assert.Empty(t, refs)
			continue
		}

		assert.NotEmpty(t, refs)

		assert.Equal(t, expected, refs[0].String())
	}
}
