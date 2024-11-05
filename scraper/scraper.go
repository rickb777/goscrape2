package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/htmlindex"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/cornelk/gotokit/log"
	"github.com/gammazero/workerpool"
	"github.com/rickb777/process/v2"
	"golang.org/x/net/proxy"
)

// Scraper contains all scraping data, starts the process and handles the concurrency.
// It includes the logic to decide what URLs to include/exclude and when to stop.
type Scraper struct {
	config  config.Config
	cookies *cookiejar.Jar
	URL     *url.URL // contains the main URL to parse, will be modified in case of a redirect

	auth   string
	client download.HttpClient

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

	s.config.SensibleDefaults()

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

	redirect, firstPageReferences, err := d.ProcessURL(ctx, firstItem)
	if err != nil {
		return err
	}

	if redirect != nil {
		s.URL = redirect // s.URL is not altered subsequently
	}

	// WorkQueue has unlimited buffering and so prevents deadlock
	workQueueIn, workQueueOut := process.WorkQueue[work.Item](32)
	refsQueue := make(chan htmlindex.Refs, s.config.Concurrency)

	pool := process.NewGroup()

	// Pool of processes to concurrently handle URL downloading.
	pool.GoNE(s.config.Concurrency, func(pid int) error {
		for {
			if pid == 0 || d.Throttle.IsNormal() {
				select {
				case <-ctx.Done():
					return nil

				case item, open := <-workQueueOut:
					if !open {
						return nil // normal 'clean' termination
					} else {
						_, moreRefs, err := d.ProcessURL(ctx, item)
						if err != nil {
							if !errors.Is(err, context.Canceled) {
								logger.Error("Failed", log.String("item", item.String()), log.Err(err))
							}
							return err
						}

						refsQueue <- moreRefs
					}
				}
			} else {
				// when throttling, do nothing for a while
				time.Sleep(500 * time.Millisecond)
			}
		}
	})

	// This goroutine is not part of the pool. It decides when to terminate based on counting
	// work done/remaining work to do. When it terminates, it closes the workQueueIn channel,
	// causing all the pool goroutines to terminate.
	go func() {
		todo := 1 // first page references
		for refs := range refsQueue {
			todo--
			for _, ref := range refs {
				next := firstItem.Derive(ref)
				if s.shouldURLBeDownloaded(next) {
					workQueueIn <- next
					todo++
				}
			}
			if todo == 0 {
				break
			}
		}
		close(workQueueIn)
	}()

	// start the ball rolling: this creates the first batch of work items
	refsQueue <- firstPageReferences

	// all the pool processes are busy until this unblocks.
	pool.Wait()
	return pool.Err()
}
