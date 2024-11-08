package download

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ignoredURLPrefixes contains a list of URL prefixes that do not need to bo adjusted.
var ignoredURLPrefixes = []string{
	"#",       // fragment
	"/#",      // fragment
	"data:",   // embedded data
	"mailto:", // mail address
}

// fixURLReferences fixes URL references to point to relative file names.
// It returns a bool that indicates that no reference needed to be fixed,
// in this case the returned HTML string will be empty.
func (d *Download) fixURLReferences(u *url.URL, doc *html.Node, index *htmlindex.Index) ([]byte, bool, error) {
	relativeToRoot := urlRelativeToRoot(u)
	if !fixHTMLNodeURLs(u, d.StartURL.Host, relativeToRoot, index) {
		return nil, false, nil
	}

	var rendered bytes.Buffer
	if err := html.Render(&rendered, doc); err != nil {
		return nil, false, fmt.Errorf("rendering html: %w", err)
	}
	if strings.Contains(rendered.String(), `html5media`) {
		logger.Debug("FOUND")
	}
	return rendered.Bytes(), true, nil
}

// fixHTMLNodeURLs processes all HTML nodes that contain URLs that need to be fixed
// to link to downloaded files. It returns whether any URLS have been fixed.
func fixHTMLNodeURLs(baseURL *url.URL, startURLHost string, relativeToRoot string, index *htmlindex.Index) (changed bool) {
	for tag, nodeInfo := range htmlindex.Nodes {
		isHyperlink := tag == atom.A

		urls := index.Nodes(tag)
		for _, nodes := range urls {
			for _, node := range nodes {
				if fixNodeURL(baseURL, nodeInfo.Attributes, node, startURLHost, isHyperlink, relativeToRoot) {
					changed = true
				}
			}
		}
	}

	return changed
}

// fixNodeURL fixes the URL references of a HTML node to point to a relative file name.
// It returns true if any attribute value bas been adjusted.
func fixNodeURL(baseURL *url.URL, attributes []string, node *html.Node, startURLHost string, isHyperlink bool, relativeToRoot string) (changed bool) {
	for i, attr := range node.Attr {
		if !slices.Contains(attributes, attr.Key) {
			continue
		}

		value := strings.TrimSpace(attr.Val)
		if value == "" {
			continue
		}

		for _, prefix := range ignoredURLPrefixes {
			if strings.HasPrefix(value, prefix) {
				return false
			}
		}

		var adjusted string

		if _, isSrcSet := htmlindex.SrcSetAttributes[attr.Key]; isSrcSet {
			adjusted = resolveSrcSetURLs(baseURL, value, startURLHost, isHyperlink, relativeToRoot)
		} else {
			adjusted = resolveURL(baseURL, value, startURLHost, relativeToRoot)
		}

		if adjusted != value { // check for no change
			attribute := &node.Attr[i]
			attribute.Val = adjusted
			changed = true

			logger.Debug("HTML node relinked",
				slog.String("value", value),
				slog.String("fixed_value", adjusted))
		}
	}

	return changed
}

func resolveSrcSetURLs(base *url.URL, srcSetValue, startURLHost string, isHyperlink bool, relativeToRoot string) string {
	// split the set of responsive images
	values := strings.Split(srcSetValue, ",")

	for i, value := range values {
		value = strings.TrimSpace(value)
		parts := strings.Split(value, " ")
		parts[0] = resolveURL(base, parts[0], startURLHost, relativeToRoot)
		values[i] = strings.Join(parts, " ")
	}

	return strings.Join(values, ", ")
}
