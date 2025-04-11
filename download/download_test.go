package download

import (
	"context"
	"github.com/rickb777/expect"
	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/stubclient"
	"github.com/rickb777/goscrape2/work"
	"github.com/spf13/afero"
	"net/http"
	"testing"
)

func TestProcessURL_200_HTML(t *testing.T) {
	page2 := `
<html>
<body>

<!--link to index with fragment-->
<a href="/#fragment">a</a>
<!--link to page with fragment-->
<a href="/sub/#fragment">a</a>
<img src="pix/photo.jpg">Photo</img>

</body>
</html>
`
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/page2/", "text/html", page2)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client:   stub,
		StartURL: mustParse("http://example.org/"),
		Auth:     "credentials",
		Fs:       afero.NewMemMapFs(),
	}

	_, result, err := d.ProcessURL(context.Background(), work.Item{URL: mustParse("https://example.org/page2/")})

	expect.Error(err).ToBeNil(t)
	expect.Number(result.StatusCode).ToBe(t, http.StatusOK)
	expect.Slice(result.References).ToHaveLength(t, 3)
	expect.Slice(result.References).ToContainAll(t,
		mustParse("https://example.org/"),
		mustParse("https://example.org/sub/"),
		mustParse("https://example.org/page2/pix/photo.jpg"))
}

func TestProcessURL_200_CSS(t *testing.T) {
	sample := `
			div#d1 { background: url(/doc/gopher.png) no-repeat; height: 155px; }
			div#d2 { background: url(food/cheese.png) no-repeat; height: 155px; }
	`
	stub := &stubclient.Client{}
	stub.GivenResponse(http.StatusOK, "https://example.org/sub/page2.css", "text/css", sample)

	d := &Download{
		Config: config.Config{
			UserAgent: "Foo/Bar",
			Header:    http.Header{"X-Extra": []string{"Hello"}},
		},
		Client:   stub,
		StartURL: mustParse("http://example.org/"),
		Auth:     "credentials",
		Fs:       afero.NewMemMapFs(),
	}

	_, result, err := d.ProcessURL(context.Background(), work.Item{URL: mustParse("https://example.org/sub/page2.css")})

	expect.Error(err).ToBeNil(t)
	expect.Number(result.StatusCode).ToBe(t, http.StatusOK)
	expect.Slice(result.References).ToHaveLength(t, 2)
	expect.Slice(result.References).ToContainAll(t,
		mustParse("https://example.org/doc/gopher.png"),
		mustParse("https://example.org/sub/food/cheese.png"))
}
