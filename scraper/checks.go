package scraper

import (
	"github.com/rickb777/goscrape2/work"
	"net/url"
)

// shouldURLBeDownloaded checks whether a page should be downloaded.
// nolint: cyclop
func (sc *Scraper) shouldURLBeDownloaded(item *url.URL, depth int) bool {
	if item.Scheme != "http" && item.Scheme != "https" {
		return false
	}

	p := item.String()
	if item.Host == sc.URL.Host {
		p = item.Path
	}
	if p == "" {
		p = "/"
	}

	if !sc.processed.AddIfAbsent(p) { // was already downloaded or checked?
		return false
	}

	if item.Host != sc.URL.Host {
		return false
	}

	if depth > sc.config.MaxDepth {
		return false
	}

	if sc.includes.Present() && !sc.includes.Matches(item, "Including URL") {
		return false
	}

	if sc.excludes.Present() && sc.excludes.Matches(item, "Skipping URL") {
		return false
	}

	return true
}

func (sc *Scraper) partitionResult(result *work.Result, depth int) {
	included := make([]*url.URL, 0, len(result.References))

	for _, ref := range result.References {
		if sc.shouldURLBeDownloaded(ref, depth) {
			included = append(included, ref)
		} else {
			result.Excluded = append(result.Excluded, ref)
		}
	}

	result.References = included
}
