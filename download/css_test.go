package download

import (
	"io"
	"log/slog"
	"net/url"
	"strings"
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

	fixtures := []struct{ input, resolved, ref string }{
		{
			input:    "url('http://localhost/uri/between/single/quote')",
			resolved: "url(../../uri/between/single/quote)",
			ref:      "http://localhost/uri/between/single/quote",
		},
		{
			input:    `url("http://localhost/uri/between/double/quote")`,
			resolved: "url(../../uri/between/double/quote)",
			ref:      "http://localhost/uri/between/double/quote",
		},
		{
			input:    "url(http://localhost/uri)",
			resolved: "url(../../uri)",
			ref:      "http://localhost/uri",
		},
		{
			input:    "url(/banner.jpg)",
			resolved: "url(../../banner.jpg)",
			ref:      "http://localhost/banner.jpg",
		},
		{
			input:    "url(../banner.jpg)",
			resolved: "url(../banner.jpg)",
			ref:      "http://localhost/css/banner.jpg",
		},
		{
			input:    "url(data:image/gif;base64,R0lGODl)",
			resolved: "url(data:image/gif;base64,R0lGODl)",
			ref:      "",
		},
		{
			input: `div#gopher {
			background: url(/doc/gopher/frontpage.png) no-repeat;
			height: 155px;
			}`,
			resolved: "url(../../doc/gopher/frontpage.png)",
			ref:      "http://localhost/doc/gopher/frontpage.png",
		},
	}

	cssURL, _ := url.Parse("http://localhost/css/x/page.css")
	for _, c := range fixtures {
		revised, refs := d.checkCSSForUrls(cssURL, []byte(c.input))

		if c.ref == "" {
			assert.Empty(t, refs)
			continue
		}

		assert.NotEmpty(t, refs)
		assert.Equal(t, c.ref, refs[0].String())

		assert.True(t, strings.Contains(string(revised), c.resolved), string(revised))
	}
}
