package document

import (
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/logger"
)

func TestCheckCSSForURLs(t *testing.T) {
	logger.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	cases := []struct{ input, resolved, ref string }{
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

	for i, c := range cases {
		revised, refs := CheckCSSForUrls(cssURL, "localhost", []byte(c.input))

		if c.ref == "" {
			expect.Slice(refs).Info(i).ToBeEmpty(t)
			continue
		}

		expect.Slice(refs).Info(i).Not().ToBeEmpty(t)
		expect.String(refs[0].String()).Info(i).ToBe(t, c.ref)

		expect.String(string(revised)).Info(i).ToContain(t, c.resolved)
	}
}
