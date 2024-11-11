package document

import (
	"bufio"
	"encoding/xml"
	"io"
	"net/url"
)

// Node describes XML nodes in an easy-to-use way.
// See https://stackoverflow.com/questions/30256729/how-to-traverse-through-xml-data-in-golang
// and https://go.dev/play/p/d9BkGclp-1.
type Node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
	Nodes   []Node     `xml:",any"`
}

func (n *Node) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Attrs = start.Attr
	type node Node

	return d.DecodeElement((*node)(n), &start)
}

type SVGDocument struct {
	u        *url.URL
	startURL *url.URL
	doc      *Node
}

func ParseSVG(u, startURL *url.URL, rdr io.Reader) (*SVGDocument, error) {
	dec := xml.NewDecoder(bufio.NewReader(rdr))

	var n Node
	err := dec.Decode(&n)
	if err != nil {
		return nil, err
	}

	return &SVGDocument{u: u, startURL: startURL, doc: &n}, nil
}
