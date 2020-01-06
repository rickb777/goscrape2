package scraper

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gorilla/css/scanner"
	"github.com/headzoo/surf/browser"
	"go.uber.org/zap"
)

var cssURLRe = regexp.MustCompile(`^url\(['"]?(.*?)['"]?\)$`)

func (s *Scraper) checkCSSForUrls(url *url.URL, buf *bytes.Buffer) *bytes.Buffer {
	m := make(map[string]string)
	str := buf.String()
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

		u, err := url.Parse(src)
		if err != nil {
			return buf
		}
		u = url.ResolveReference(u)

		// Create new URL object and a pointer to it.
		cssPath := *url
		cssPath.Path = path.Dir(cssPath.Path) + "/"

		img := browser.NewImageAsset(u, "", "", "")
		s.imagesQueue = append(s.imagesQueue, &img.DownloadableAsset)

		resolved := s.resolveURL(&cssPath, src, false, "")
		m[token.Value] = resolved
	}

	if len(m) == 0 {
		return buf
	}

	for ori, filePath := range m {
		fixed := fmt.Sprintf("url(%s)", filePath)
		str = strings.Replace(str, ori, fixed, -1)
		s.log.Debug("CSS Element relinked", zap.String("URL", ori), zap.String("Fixed", fixed))
	}

	return bytes.NewBufferString(str)
}
