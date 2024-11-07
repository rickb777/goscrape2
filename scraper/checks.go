package scraper

import (
	"github.com/cornelk/goscrape/work"
	"net/url"
)

// shouldURLBeDownloaded checks whether a page should be downloaded.
// nolint: cyclop
func (s *Scraper) shouldURLBeDownloaded(item *url.URL, depth uint) bool {
	if item.Scheme != "http" && item.Scheme != "https" {
		return false
	}

	p := item.String()
	if item.Host == s.URL.Host {
		p = item.Path
	}
	if p == "" {
		p = "/"
	}

	if !s.processed.AddIfAbsent(p) { // was already downloaded or checked?
		return false
	}

	if item.Host != s.URL.Host {
		return false
	}

	if depth >= s.config.MaxDepth {
		return false
	}

	if s.includes != nil && !s.includes.Matches(item, "Including URL") {
		return false
	}

	if s.excludes != nil && s.excludes.Matches(item, "Skipping URL") {
		return false
	}

	return true
}

func (s *Scraper) partitionResult(result *work.Result, depth uint) {
	var included []*url.URL
	for _, ref := range result.References {
		if s.shouldURLBeDownloaded(ref, depth) {
			included = append(included, ref)
		} else {
			result.Excluded = append(result.Excluded, ref)
		}
	}
	result.References = included
}
