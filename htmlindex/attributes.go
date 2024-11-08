package htmlindex

import "golang.org/x/net/html/atom"

// nodeAttributeParser returns the URL values of the attribute of the node and
// whether the attribute has been processed.
type nodeAttributeParser func(attribute, value string) ([]string, bool)

type Node struct {
	Attributes []string
	parser     nodeAttributeParser
}

const (
	background = "background"
	href       = "href"
	dataSrc    = "data-src"
	src        = "src"
	poster     = "poster"

	// sets
	dataSrcSet = "data-srcset"
	srcSet     = "srcset"
)

// Nodes describes the HTML tags and their attributes that can contain URL.
// See https://html.spec.whatwg.org/multipage/indices.html#attributes-3
// and https://html.spec.whatwg.org/multipage/indices.html#elements-3
// Not yet present: style attribute can contain CSS links
var Nodes = map[atom.Atom]Node{
	atom.A: {
		Attributes: []string{href},
	},
	atom.Area: {
		Attributes: []string{href},
	},
	atom.Base: {
		Attributes: []string{href},
	},
	atom.Audio: {
		Attributes: []string{src},
	},
	atom.Body: {
		Attributes: []string{background},
	},
	atom.Embed: {
		Attributes: []string{src},
	},
	atom.Iframe: {
		Attributes: []string{src},
	},
	atom.Img: {
		Attributes: []string{src, dataSrc, srcSet, dataSrcSet},
		parser:     srcSetValueSplitter,
	},
	atom.Input: {
		Attributes: []string{src},
	},
	atom.Link: {
		Attributes: []string{href},
	},
	atom.Script: {
		Attributes: []string{src},
	},
	atom.Source: {
		Attributes: []string{src},
	},
	atom.Video: {
		Attributes: []string{poster},
	},
}

// SrcSetAttributes contains the attributes that contain srcset values.
var SrcSetAttributes = map[string]struct{}{
	dataSrcSet: {},
	srcSet:     {},
}
