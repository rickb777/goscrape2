package document

import (
	"bytes"
	"fmt"
	"github.com/beevik/etree"
	"github.com/rickb777/goscrape2/work"
	"io"
	"net/url"
)

type SVGDocument struct {
	etree.Document
	u        *url.URL
	startURL *url.URL
}

func ParseSVG(u, startURL *url.URL, rdr io.Reader) (*SVGDocument, error) {
	doc := etree.NewDocument()
	_, err := doc.ReadFrom(rdr)
	if err != nil {
		return nil, err
	}

	return &SVGDocument{Document: *doc, u: u, startURL: startURL}, nil
}

// FixURLReferences fixes URL references to point to relative file names.
// It returns a bool that indicates that no reference needed to be fixed,
// in this case the returned HTML string will be empty.
func (d *SVGDocument) FixURLReferences() ([]byte, bool, work.Refs, error) {
	relativeToRoot := urlRelativeToRoot(d.u)
	//if !fixHTMLNodeURLs(d.u, d.startURL.Host, relativeToRoot, d.index) {
	//	return nil, false, nil
	//}

	var links []string
	walkXML(&d.Element, func(node *etree.Element) bool {
		for i, a := range node.Attr {
			if isLink(a) {
				links = append(links, a.Value)
				a.Value = relativeToRoot + a.Value
				node.Attr[i] = a
			}
		}
		return true
	})

	var result work.Refs

	var rendered bytes.Buffer
	_, err := d.WriteTo(&rendered)
	if err != nil {
		return nil, false, nil, fmt.Errorf("rendering SVG: %w", err)
	}

	return rendered.Bytes(), true, result, nil
}

func isLink(a etree.Attr) bool {
	return (a.Space == "" || a.Space == "xlink") && a.Key == "href"
}

func walkXML(node *etree.Element, f func(*etree.Element) bool) {
	if f(node) {
		for _, c := range node.ChildElements() {
			walkXML(c, f)
		}
	}
}
