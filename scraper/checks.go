package scraper

import (
	"net/url"

	"github.com/cornelk/goscrape/work"
	"github.com/cornelk/gotokit/log"
)

// shouldURLBeDownloaded checks whether a page should be downloaded.
// nolint: cyclop
func (s *Scraper) shouldURLBeDownloaded(item work.Item, isAsset bool) bool {
	if item.URL.Scheme != "http" && item.URL.Scheme != "https" {
		return false
	}

	p := item.URL.String()
	if item.URL.Host == s.URL.Host {
		p = item.URL.Path
	}
	if p == "" {
		p = "/"
	}

	if _, ok := s.processed[p]; ok { // was already downloaded or checked?
		if item.URL.Fragment != "" {
			return false
		}
		return false
	}

	s.processed[p] = struct{}{}

	if !isAsset {
		if item.URL.Host != s.URL.Host {
			s.logger.Debug("Skipping external host page", log.String("url", item.URL.String()))
			return false
		}

		if s.config.MaxDepth != 0 && item.Depth == s.config.MaxDepth {
			s.logger.Debug("Skipping too deep level page", log.String("url", item.URL.String()))
			return false
		}
	}

	if s.includes != nil && !s.isURLIncluded(item.URL) {
		return false
	}
	if s.excludes != nil && s.isURLExcluded(item.URL) {
		return false
	}

	s.logger.Debug("New URL to download", log.String("url", item.URL.String()))
	return true
}

func (s *Scraper) isURLIncluded(url *url.URL) bool {
	for _, re := range s.includes {
		if re.MatchString(url.Path) {
			s.logger.Info("Including URL",
				log.String("url", url.String()),
				log.Stringer("included_expression", re))
			return true
		}
	}
	return false
}

func (s *Scraper) isURLExcluded(url *url.URL) bool {
	for _, re := range s.excludes {
		if re.MatchString(url.Path) {
			s.logger.Info("Skipping URL",
				log.String("url", url.String()),
				log.Stringer("excluded_expression", re))
			return true
		}
	}
	return false
}
