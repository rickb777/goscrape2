package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
	"github.com/rickb777/process/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/work"
	"github.com/gammazero/workerpool"
	"golang.org/x/net/proxy"
)

// Scraper contains all scraping data, starts the process and handles the concurrency.
// It includes the logic to decide what URLs to include/exclude and when to stop.
type Scraper struct {
	config  config.Config
	cookies *cookiejar.Jar
	URL     *url.URL // contains the main URL to parse, will be modified in case of a redirect

	auth   string
	client *http.Client

	includes filter.Filter
	excludes filter.Filter

	workers *workerpool.WorkerPool

	// key is the URL of page or asset
	processed *work.Set[string]
}

// New creates a new Scraper instance.
// nolint: funlen
func New(cfg config.Config) (*Scraper, error) {
	var errs []error

	u, err := url.Parse(cfg.URL)
	if err != nil {
		errs = append(errs, err)
	}
	u.Fragment = ""

	includes, err := filter.New(cfg.Includes)
	if err != nil {
		errs = append(errs, err)
	}

	excludes, err := filter.New(cfg.Excludes)
	if err != nil {
		errs = append(errs, err)
	}

	proxyURL, err := url.Parse(cfg.Proxy)
	if err != nil {
		errs = append(errs, err)
	}

	if errs != nil {
		return nil, errors.Join(errs...)
	}

	if u.Scheme == "" {
		u.Scheme = "http" // if no URL scheme was given default to http
	}

	cookies, err := createCookieJar(u, cfg.Cookies)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar:     cookies,
		Timeout: cfg.Timeout,
	}

	if cfg.Proxy != "" {
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("creating proxy from URL: %w", err)
		}

		dialerCtx, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, errors.New("proxy dialer is not a context dialer")
		}

		client.Transport = &http.Transport{
			DialContext: dialerCtx.DialContext,
		}
	}

	s := &Scraper{
		config:  cfg,
		cookies: cookies,
		URL:     u,

		client: client,

		includes: includes,
		excludes: excludes,

		workers:   workerpool.New(cfg.Concurrency),
		processed: work.NewSet[string](),
	}

	if s.config.Username != "" {
		s.auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(s.config.Username+":"+s.config.Password))
	}

	return s, nil
}

// Start starts the scraping.
func (s *Scraper) Start(ctx context.Context) error {
	err := download.CreateDirectory(s.config.OutputDirectory)
	if err != nil {
		return err
	}

	firstItem := work.Item{URL: s.URL}

	if !s.shouldURLBeDownloaded(firstItem) {
		return errors.New("start page is excluded from downloading")
	}

	d := &download.Download{
		Config:   s.config,
		Cookies:  s.cookies,
		StartURL: s.URL,
		Auth:     s.auth,
		Client:   s.client,
	}

	redir, references, err := d.ProcessURL(ctx, firstItem)
	if err != nil {
		return err
	}

	if redir != nil {
		s.URL = redir // s.URL is not altered by any subsequent URLs
	}

	workQueue, queuedItems := process.WorkQueue[work.Item](32)

	for _, ref := range references {
		next := firstItem.Derive(ref)
		if s.shouldURLBeDownloaded(next) {
			workQueue <- next
		}
	}

	pool := process.NewGroup()

	pool.GoNE(s.config.Concurrency, func(_ int) error {
		for item := range queuedItems {
			_, moreRefs, err := d.ProcessURL(ctx, item)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					logger.Error("Failed", log.String("item", item.String()), log.Err(err))
				}
				return err
			}

			for _, ref := range moreRefs {
				next := firstItem.Derive(ref)
				if s.shouldURLBeDownloaded(next) {
					workQueue <- next
				}
			}
		}
		return nil
	})

	pool.Wait()
	return nil
}
