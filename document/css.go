package document

import (
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gorilla/css/scanner"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/work"
)

var cssURLRe = regexp.MustCompile(`^url\(['"]?(.*?)['"]?\)$`)

func CheckCSSForUrls(cssURL *url.URL, startURLHost string, data []byte) ([]byte, work.Refs) {
	var refs work.Refs
	urls := make(map[string]string)
	css := string(data)
	scan := scanner.New(css)

	for {
		token := scan.Next()
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

		resolved, err := cssURL.Parse(src)
		if err != nil {
			logger.Logger.Error("Parsing URL failed",
				slog.String("url", src),
				slog.Any("error", err))
			continue
		}

		refs = append(refs, resolved)

		cssPath := *cssURL
		cssPath.Path = path.Dir(cssPath.Path) + "/"

		urls[token.Value] = resolveURL(&cssPath, src, startURLHost, "")
	}

	if len(urls) == 0 {
		return data, refs // nothing more needs doing
	}

	// fix all the urls in the CSS source
	for original, filePath := range urls {
		fixed := fmt.Sprintf("url(%s)", filePath)
		css = strings.ReplaceAll(css, original, fixed)
		logger.Debug("CSS element relinked", slog.String("url", original), slog.String("fixed", fixed))
	}

	return []byte(css), refs
}
