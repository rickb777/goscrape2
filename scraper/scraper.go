package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/cornelk/goscrape/download/throttle"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/filter"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/utc"
	"github.com/cornelk/goscrape/work"
	"github.com/rickb777/process/v2"
	"github.com/spf13/afero"
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
	fs     afero.Fs // filesystem

	includes filter.Filter
	excludes filter.Filter

	// key is the URL of page or asset
	processed *work.Set[string]

	// ETagsDB stores ETags (hashes of file state) for each URL
	ETagsDB *db.DB
}

//-------------------------------------------------------------------------------------------------

// New creates a new Scraper instance.
// nolint: funlen
func New(cfg config.Config, fs afero.Fs) (*Scraper, error) {
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
		fs:     fs, // filesystem can be replaced with in-memory filesystem for testing

		includes: includes,
		excludes: excludes,

		processed: work.NewSet[string](),
	}

	if s.config.Username != "" {
		s.auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(s.config.Username+":"+s.config.Password))
	}

	return s, nil
}

//-------------------------------------------------------------------------------------------------

// Start starts the scraping.
func (s *Scraper) Start(ctx context.Context) error {
	err := ioutil.CreateDirectory(s.fs, s.config.OutputDirectory)
	if err != nil {
		return err
	}

	s.config.SensibleDefaults()

	firstItem := work.Item{URL: s.URL}

	if !s.shouldURLBeDownloaded(firstItem.URL, 0) {
		return errors.New("start page is excluded from downloading")
	}

	d := &download.Download{
		Config:    s.config,
		Cookies:   s.cookies,
		ETagsDB:   s.ETagsDB,
		StartURL:  s.URL,
		Auth:      s.auth,
		Client:    s.client,
		Fs:        s.fs,
		Lockdown:  throttle.New(0, 10*time.Second, 2*time.Second),
		LoopDelay: throttle.New(s.config.LoopDelay, time.Millisecond, time.Millisecond/2),
	}

	redirect, firstResult, err := d.ProcessURL(ctx, firstItem)
	if err != nil {
		return err
	}

	if redirect != nil {
		s.URL = redirect // s.URL is not altered subsequently
	}

	// WorkQueue has unlimited buffering and so prevents deadlock
	workQueueIn, workQueueOut := process.WorkQueue[work.Item](32)
	results := make(chan work.Result, s.config.Concurrency)

	pool := process.NewGroup()

	// Pool of processes to concurrently handle URL downloading.
	pool.GoNE(s.config.Concurrency, func(pid int) error {
		for {
			if pid == 0 || d.Lockdown.IsNormal() {
				select {
				case <-ctx.Done():
					return nil

				case item, open := <-workQueueOut:
					if !open {
						return nil // normal 'clean' termination
					} else {
						_, result, err := d.ProcessURL(ctx, item)
						if err != nil {
							if !errors.Is(err, context.Canceled) {
								logger.Error("Failed", slog.String("item", item.String()), slog.Any("error", err))
							}
							return err
						}

						logResult(result)

						results <- *result
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
		for result := range results {
			todo--
			newDepth := result.Item.Depth + 1
			s.partitionResult(&result, newDepth)
			logger.Debug("Partitioned", slog.Any("item", result.Item), slog.Any("include", result.References), slog.Any("exclude", result.Excluded))
			for _, ref := range result.References {
				workQueueIn <- work.Item{URL: ref, Referrer: result.Item.URL, Depth: newDepth}
			}
			todo += len(result.References)
			if todo == 0 {
				break
			}
		}
		close(workQueueIn)
	}()

	// start the ball rolling: this creates the first batch of work items
	results <- *firstResult

	// all the pool processes are busy until this unblocks.
	pool.Wait()
	return pool.Err()
}

//-------------------------------------------------------------------------------------------------

func logResult(result *work.Result) {
	// using a func result so that it can be applied transparently to the major method call sites, above
	var args = []any{
		slog.String("url", result.Item.URL.String()),
		slog.Int("depth", result.Item.Depth),
		slog.Int("code", result.StatusCode),
		slog.String("took", timeTaken(result.Item.StartTime)),
	}
	if result.ContentLength > 0 && result.ContentLength != result.FileSize {
		args = append(args, slog.Int64("length", result.ContentLength))
	}
	if result.FileSize > 0 {
		args = append(args, slog.Int64("fileSize", result.FileSize))
	}
	if result.Gzip {
		args = append(args, slog.String("enc", "gzip"))
	}
	logger.Log(chooseLevel(result.StatusCode), statusText(result.StatusCode), args...)
}

func timeTaken(before time.Time) string {
	return utc.Now().Sub(before).Round(time.Millisecond).String()
}

func chooseLevel(statusCode int) slog.Level {
	if statusCode >= 400 {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

func statusText(statusCode int) string {
	if statusCode == http.StatusTeapot {
		return "Skipped"
	}
	return http.StatusText(statusCode)
}
