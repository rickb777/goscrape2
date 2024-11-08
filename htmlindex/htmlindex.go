package htmlindex

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Refs []*url.URL

// Index provides an index for all HTML tags of relevance for scraping.
type Index struct {
	// key is HTML tag, value is a map of all its urls and the HTML nodes for it
	data map[atom.Atom]map[string][]*html.Node
}

// New returns a new index.
func New() *Index {
	return &Index{
		data: make(map[atom.Atom]map[string][]*html.Node),
	}
}

// Index the given HTML document.
func (h *Index) Index(baseURL *url.URL, node *html.Node) {
	if explicitBaseURL := h.findBaseHref(node); explicitBaseURL != nil {
		h.indexChildren(explicitBaseURL, node)
	} else {
		h.indexChildren(baseURL, node)
	}
}

// findBaseHref finds the URL from the <base href="..."/> element, if there is one.
func (h *Index) findBaseHref(node *html.Node) (baseURL *url.URL) {
	if node.FirstChild == nil {
		return nil
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		if child.DataAtom == atom.Base && node.DataAtom == atom.Head {
			var references []string

			info, ok := Nodes[atom.Base]
			if ok {
				references = nodeAttributeURLs(nil, child, info.parser, info.Attributes...)
			}

			if len(references) == 1 {
				if newBase, err := url.Parse(references[0]); err == nil {
					return newBase
				} else {
					return nil
				}
			}
		}

		if baseURL = h.findBaseHref(child); baseURL != nil {
			return baseURL
		}
	}

	return baseURL
}

// indexChildren indexes all the children of node. References are resolved relative to baseURL.
func (h *Index) indexChildren(baseURL *url.URL, node *html.Node) {
	if node.FirstChild == nil {
		return
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		var references []string

		info, ok := Nodes[child.DataAtom]
		if ok {
			references = nodeAttributeURLs(baseURL, child, info.parser, info.Attributes...)
		}

		m, ok := h.data[child.DataAtom]
		if !ok {
			m = map[string][]*html.Node{}
			h.data[child.DataAtom] = m
		}

		for _, reference := range references {
			m[reference] = append(m[reference], child)
		}

		h.indexChildren(baseURL, child)
	}
}

// URLs returns all URLs of the references found for a specific tag.
func (h *Index) URLs(tag atom.Atom) (Refs, error) {
	m, ok := h.data[tag]
	if !ok {
		return nil, nil
	}

	data := make([]string, 0, len(m))
	for key := range m {
		data = append(data, key)
	}
	sort.Strings(data)

	urls := make(Refs, 0, len(m))
	for _, fullURL := range data {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("parsing URL '%s': %w", fullURL, err)
		}
		urls = append(urls, u)
	}

	return urls, nil
}

// Nodes returns a map of all URLs and their HTML nodes.
func (h *Index) Nodes(tag atom.Atom) map[string][]*html.Node {
	m, ok := h.data[tag]
	if ok {
		return m
	}
	return map[string][]*html.Node{}
}

// nodeAttributeURLs returns resolved URLs based on the base URL and the HTML node attribute values.
func nodeAttributeURLs(baseURL *url.URL, node *html.Node,
	parser nodeAttributeParser, attributeName ...string) []string {

	var results []string

	for _, attr := range node.Attr {
		var process bool
		for _, name := range attributeName {
			if attr.Key == name {
				process = true
				break
			}
		}
		if !process {
			continue
		}

		var references []string
		var parserHandled bool

		if parser != nil {
			references, parserHandled = parser(attr.Key, strings.TrimSpace(attr.Val))
		}
		if parser == nil || !parserHandled {
			references = append(references, strings.TrimSpace(attr.Val))
		}

		for _, reference := range references {
			ur, err := url.Parse(reference)
			if err != nil {
				continue
			}

			if baseURL != nil {
				ur = baseURL.ResolveReference(ur)
			}
			results = append(results, ur.String())
		}
	}

	return results
}

// srcSetValueSplitter returns the URL values of the srcset attribute of img nodes.
func srcSetValueSplitter(attribute, attributeValue string) ([]string, bool) {
	if _, isSrcSet := SrcSetAttributes[attribute]; !isSrcSet {
		return nil, false
	}

	// split the set of responsive images
	values := strings.Split(attributeValue, ",")

	for i, value := range values {
		value = strings.TrimSpace(value)
		// remove the width in pixels after the url
		values[i], _, _ = strings.Cut(value, " ")
	}

	return values, true
}
