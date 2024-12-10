package document

import (
	"github.com/rickb777/goscrape2/mapping"
	urlpkg "net/url"
	"path"
	"strings"
)

func resolveURL(base *urlpkg.URL, reference, startURLHost, relativeToRoot string) string {
	url, err := urlpkg.Parse(reference)
	if err != nil {
		return ""
	}

	if url.Host != "" && url.Host != startURLHost {
		return reference // points to a different website - leave unchanged
	}

	resolvedURL := base.ResolveReference(url)

	if resolvedURL.Host == startURLHost {
		resolvedURL.Path = urlRelativeToOther(resolvedURL, base)
		relativeToRoot = ""
	}

	resolvedURL.Host = ""   // remove host
	resolvedURL.Scheme = "" // remove http/https
	resolved := resolvedURL.String()

	if resolved == "" {
		resolved = "/" // website root
	} else {
		if resolved[0] == '/' && len(relativeToRoot) > 0 {
			resolved = relativeToRoot + resolved[1:]
		} else {
			resolved = relativeToRoot + resolved
		}
	}

	resolved = strings.TrimPrefix(resolved, "/")
	return resolved
}

func urlRelativeToRoot(url *urlpkg.URL) string {
	var rel string
	splits := strings.Split(url.Path, "/")
	for i := range splits {
		if (len(splits[i]) > 0) && (i < len(splits)-1) {
			rel += "../"
		}
	}
	return rel
}

func urlRelativeToOther(src, base *urlpkg.URL) string {
	srcSplits := strings.Split(src.Path, "/")
	baseSplits := strings.Split(mapping.GetPageFilePath(base), "/")

	for {
		if len(srcSplits) == 0 || len(baseSplits) == 0 {
			break
		}
		if len(srcSplits[0]) == 0 {
			srcSplits = srcSplits[1:]
			continue
		}
		if len(baseSplits[0]) == 0 {
			baseSplits = baseSplits[1:]
			continue
		}

		if srcSplits[0] == baseSplits[0] {
			srcSplits = srcSplits[1:]
			baseSplits = baseSplits[1:]
		} else {
			break
		}
	}

	var upLevels string
	for i, split := range baseSplits {
		if split == "" {
			continue
		}
		// Page filename is not a level.
		if i == len(baseSplits)-1 {
			break
		}
		upLevels += "../"
	}

	return upLevels + path.Join(srcSplits...)
}
