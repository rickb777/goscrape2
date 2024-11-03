package download

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
	"github.com/gorilla/css/scanner"
)

var cssURLRe = regexp.MustCompile(`^url\(['"]?(.*?)['"]?\)$`)

func (d *Download) checkCSSForUrls(cssURL *url.URL, data []byte) ([]byte, []*url.URL) {
	var refs []*url.URL
	urls := make(map[string]string)
	str := string(data)
	css := scanner.New(str)

	for {
		token := css.Next()
		if token.Type == scanner.TokenEOF || token.Type == scanner.TokenError {
			break
		}

		if token.Type != scanner.TokenURI {
			continue
		}

		match := cssURLRe.FindStringSubmatch(token.Value)
		if match == nil {
			continue
		}

		src := match[1]
		if strings.HasPrefix(strings.ToLower(src), "data:") {
			continue // skip embedded data
		}

		u, err := cssURL.Parse(src)
		if err != nil {
			logger.Logger.Error("Parsing URL failed",
				log.String("url", src),
				log.Err(err))
			continue
		}

		u = cssURL.ResolveReference(u)

		refs = append(refs, u)

		cssPath := *cssURL
		cssPath.Path = path.Dir(cssPath.Path) + "/"
		resolved := resolveURL(&cssPath, src, d.StartURL.Host, false, "")
		urls[token.Value] = resolved
	}

	if len(urls) == 0 {
		return data, refs
	}

	for original, filePath := range urls {
		fixed := fmt.Sprintf("url(%s)", filePath)
		str = strings.ReplaceAll(str, original, fixed)
		logger.Debug("CSS element relinked",
			log.String("url", original),
			log.String("fixed_url", fixed))
	}

	return []byte(str), refs
}
