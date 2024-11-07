package scraper

import (
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"log/slog"
)

// shouldURLBeDownloaded checks whether a page should be downloaded.
// nolint: cyclop
func (s *Scraper) shouldURLBeDownloaded(item work.Item) bool {
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

	if !s.processed.AddIfAbsent(p) { // was already downloaded or checked?
		return false
	}

	if item.URL.Host != s.URL.Host {
		logger.Debug("Skipping external host page", slog.String("url", item.URL.String()))
		return false
	}

	if item.Depth >= s.config.MaxDepth {
		logger.Debug("Skipping too deep level page", slog.String("url", item.URL.String()))
		return false
	}

	if s.includes != nil && !s.includes.Matches(item.URL, "Including URL") {
		return false
	}

	if s.excludes != nil && s.excludes.Matches(item.URL, "Skipping URL") {
		return false
	}

	logger.Debug("New URL to download", slog.String("url", item.URL.String()))
	return true
}
