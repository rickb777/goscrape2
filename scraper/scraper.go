package scraper

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/work"
	"golang.org/x/net/proxy"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
)

// Scraper contains all scraping data.
type Scraper struct {
	config  config.Config
	cookies *cookiejar.Jar
	URL     *url.URL // contains the main URL to parse, will be modified in case of a redirect

	auth   string
	client *http.Client

	includes []*regexp.Regexp
	excludes []*regexp.Regexp

	// key is the URL of page or asset
	processed work.Set[string]

	imagesQueue []*url.URL
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

	includes, err := compileRegexps(cfg.Includes)
	if err != nil {
		errs = append(errs, err)
	}

	excludes, err := compileRegexps(cfg.Excludes)
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
	var workQueue []work.Item

	if !s.shouldURLBeDownloaded(firstItem, false) {
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
		s.URL = redir
	}

	for _, ref := range references {
		next := firstItem.Derive(ref)
		if s.shouldURLBeDownloaded(next, false) {
			workQueue = append(workQueue, next)
		}
	}

	for len(workQueue) > 0 {
		item := workQueue[0]
		workQueue = workQueue[1:]

		_, references, err = d.ProcessURL(ctx, item)
		if err != nil && errors.Is(err, context.Canceled) {
			return err
		}

		for _, ref := range references {
			next := item.Derive(ref)
			if s.shouldURLBeDownloaded(next, false) {
				workQueue = append(workQueue, next)
			}
		}
	}

	return nil
}

// compileRegexps compiles the given regex strings to regular expressions
// to be used in the include and exclude filters.
func compileRegexps(regexps []string) ([]*regexp.Regexp, error) {
	var errs []error
	var compiled []*regexp.Regexp

	for _, exp := range regexps {
		re, err := regexp.Compile(exp)
		if err == nil {
			compiled = append(compiled, re)
		} else {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return compiled, nil
}
