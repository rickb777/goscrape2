package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	urlpkg "net/url"
	"time"

	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/download"
	"github.com/rickb777/goscrape2/download/throttle"
	"github.com/rickb777/goscrape2/filter"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/utc"
	"github.com/rickb777/goscrape2/work"
	"github.com/rickb777/process/v2"
	"github.com/spf13/afero"
	"golang.org/x/net/proxy"
)

// Scraper contains all scraping data, starts the process and handles the concurrency.
// It includes the logic to decide what URLs to include/exclude and when to stop.
type Scraper struct {
	config  config.Config
	cookies *cookiejar.Jar
	URL     *urlpkg.URL // contains the main URL to parse, will be modified in case of a redirect

	auth   string
	Client download.HttpClient
	Fs     afero.Fs // filesystem

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
func New(cfg config.Config, url *urlpkg.URL, fs afero.Fs) (*Scraper, error) {
	var errs []error

	url.Fragment = ""

	includes, err := filter.New(cfg.Includes)
	if err != nil {
		errs = append(errs, err)
	}

	excludes, err := filter.New(cfg.Excludes)
	if err != nil {
		errs = append(errs, err)
	}

	proxyURL, err := urlpkg.Parse(cfg.Proxy)
	if err != nil {
		errs = append(errs, err)
	}

	if errs != nil {
		return nil, errors.Join(errs...)
	}

	if url.Scheme == "" {
		url.Scheme = "http" // if no URL scheme was given default to http
	}

	cookies, err := createCookieJar(url, cfg.Cookies)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar:     cookies,
		Timeout: cfg.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
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
		URL:     url,

		Client: client,
		Fs:     fs, // filesystem can be replaced with in-memory filesystem for testing

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

func (sc *Scraper) Downloader() *download.Download {
	sc.config.SensibleDefaults()

	return &download.Download{
		Config:    sc.config,
		Cookies:   sc.cookies,
		ETagsDB:   sc.ETagsDB,
		StartURL:  sc.URL,
		Auth:      sc.auth,
		Client:    sc.Client,
		Fs:        afero.NewBasePathFs(sc.Fs, sc.URL.Host),
		Lockdown:  throttle.New(0, 10*time.Second, 2*time.Second),
		LoopDelay: throttle.New(sc.config.LoopDelay, time.Millisecond, time.Millisecond/2),
	}
}

//-------------------------------------------------------------------------------------------------

// Start starts the scraping.
func (sc *Scraper) Start(ctx context.Context) error {
	d := sc.Downloader()

	firstItem := work.Item{URL: sc.URL}

	if !sc.shouldURLBeDownloaded(firstItem.URL, 0) {
		return fmt.Errorf("start page is excluded from downloading: %s", firstItem.URL)
	}

	redirect, firstResult, err := d.ProcessURL(ctx, firstItem)
	if err != nil {
		return err
	}

	switch firstResult.StatusCode {
	case http.StatusOK, http.StatusNotModified, http.StatusTeapot:
		if redirect != nil {
			sc.URL = redirect // sc.URL is not altered subsequently
		}

	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		sc.URL = firstResult.References[0]

	default:
		return fmt.Errorf("start page failed: %d %s", firstResult.StatusCode, http.StatusText(firstResult.StatusCode))
	}

	// WorkQueue has unlimited buffering and so prevents deadlock
	workQueueIn, workQueueOut := process.WorkQueue[work.Item](32)
	results := make(chan work.Result, sc.config.Concurrency)

	pool := process.NewGroup()

	// Pool of processes to concurrently handle URL downloading.
	pool.GoNE(sc.config.Concurrency, func(pid int) error {
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
			sc.partitionResult(&result, newDepth)
			logger.Debug("Partitioned", slog.Any("item", result.Item), slog.Any("include", result.References), slog.Any("exclude", result.Excluded))
			for _, ref := range result.References {
				u := absoluteURL(ref, result)
				workQueueIn <- work.Item{URL: u, Referrer: result.Item.URL, Depth: newDepth}
			}
			todo += len(result.References)
			if todo == 0 {
				break
			}
		}
		close(workQueueIn)
	}()

	// start the ball rolling: this creates the first batch of work items
	logResult(firstResult)
	results <- *firstResult

	// all the pool processes are busy until this unblocks.
	pool.Wait()
	return pool.Err()
}

func absoluteURL(u *urlpkg.URL, result work.Result) *urlpkg.URL {
	if u.Scheme == "" {
		u.Scheme = result.URL.Scheme
	} else if u.Scheme == "http" && result.URL.Scheme == "https" {
		logger.Warn("HTTPS downgraded",
			slog.Any("url", result.URL),
			slog.Int("code", result.StatusCode),
			slog.Any("location", u))
	}

	if u.Host == "" {
		u.Host = result.URL.Host
	}

	return u
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
	if result.IsRedirect() {
		args = append(args, slog.Any("location", result.References[0]))
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
	if statusCode == http.StatusTeapot {
		return slog.LevelInfo
	} else if statusCode >= 400 {
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
