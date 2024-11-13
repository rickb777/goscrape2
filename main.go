package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/spf13/afero"
	"log/slog"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/images"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/scraper"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type arguments struct {
	Include []string `arg:"-i,--include" help:"only include URLs that match a regular expression"`
	Exclude []string `arg:"-x,--exclude" help:"exclude URLs that match a regular expression"`
	Output  string   `arg:"-o,--output" help:"output directory to write files to"`

	URLs []string `arg:"positional"`

	Concurrency  int64         `arg:"-c,--concurrency" help:"the number of concurrent downloads" default:"1"`
	Depth        int64         `arg:"-d,--depth" help:"download depth limit, 0 for unlimited" default:"10"`
	ImageQuality int64         `arg:"-q,--imagequality" help:"image quality reduction, 0 to disable re-encoding, maximum 99"`
	Timeout      time.Duration `arg:"-t,--timeout" help:"time limit (with units, e.g. 1s) for each HTTP request to connect and read the response" default:"30s"`
	LoopDelay    time.Duration `arg:"--loopdelay" help:"delay (with units, e.g. 1s) used between any two downloads" default:"0s"`
	RetryDelay   time.Duration `arg:"--retrydelay" help:"initial delay (with units, e.g. 1s) used when retrying any download; this adds to the loop delay and grows exponentially when retrying" default:"10s"`
	LaxAge       time.Duration `arg:"--laxage" help:"adds to the 'expires' timestamp specified by the origin server, or creates one if absent; if the origin is too conservative, this helps when doing successive runs" default:"0s"`
	Tries        int64         `arg:"-n,--tries" help:"the number of tries to download each file if the server gives a 5xx error" default:"1"`

	Serve      string `arg:"-s,--serve" help:"serve the website using a webserver"`
	ServerPort int16  `arg:"-r,--serverport" help:"port to use for the webserver" default:"8080"`

	CookieFile     string `arg:"--cookiefile" help:"file containing the cookie content"`
	SaveCookieFile string `arg:"--savecookiefile" help:"file to save the cookie content"`

	Headers   []string `arg:"-h,--header" help:"HTTP header to use for scraping"`
	Proxy     string   `arg:"-p,--proxy" help:"HTTP proxy to use for scraping"`
	User      string   `arg:"-u,--user" help:"user[:password] to use for HTTP authentication"`
	UserAgent string   `arg:"-a,--useragent" help:"user agent to use for scraping"`

	Verbose bool `arg:"-v,--verbose" help:"verbose output"`
	Debug   bool `arg:"-z,--debug" help:"debug output"`
}

func (arguments) Description() string {
	return "Scrape a website and create an offline browsable version on the disk.\n"
}

func (arguments) Version() string {
	return fmt.Sprintf("formatVersion: %s\n", formatVersion(version, commit, date))
}

func main() {
	args, err := readArguments()
	if err != nil {
		fmt.Printf("Reading arguments failed: %s\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	//ctx := app.Context() // provides signal handler cancellation

	logger.Logger = createLogger(args)

	if args.Serve != "" {
		if err := runServer(ctx, args); err != nil {
			fmt.Printf("Server execution error: %s\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runScraper(ctx, args); err != nil {
		fmt.Printf("Scraping execution error: %s\n", err)
		os.Exit(1)
	}
}

func readArguments() (arguments, error) {
	var args arguments
	parser, err := arg.NewParser(arg.Config{}, &args)
	if err != nil {
		return arguments{}, fmt.Errorf("creating argument parser: %w", err)
	}

	if err = parser.Parse(os.Args[1:]); err != nil {
		switch {
		case errors.Is(err, arg.ErrHelp):
			parser.WriteHelp(os.Stdout)
			os.Exit(0)
		case errors.Is(err, arg.ErrVersion):
			fmt.Println(args.Version())
			os.Exit(0)
		}

		return arguments{}, fmt.Errorf("parsing arguments: %w", err)
	}

	if len(args.URLs) == 0 && args.Serve == "" {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	return args, nil
}

func runScraper(ctx context.Context, args arguments) error {
	if len(args.URLs) == 0 {
		return nil
	}

	var username, password string
	if args.User != "" {
		sl := strings.Split(args.User, ":")
		username = sl[0]
		if len(sl) > 1 {
			password = sl[1]
		}
	}

	imageQuality := args.ImageQuality
	if args.ImageQuality < 0 || args.ImageQuality >= 100 {
		imageQuality = 0
	}

	cookies, err := readCookieFile(args.CookieFile)
	if err != nil {
		return fmt.Errorf("reading cookie: %w", err)
	}

	cfg := config.Config{
		Includes: args.Include,
		Excludes: args.Exclude,

		Concurrency:  int(args.Concurrency),
		MaxDepth:     uint(args.Depth),
		ImageQuality: images.ImageQuality(imageQuality),
		Timeout:      args.Timeout,
		LoopDelay:    args.LoopDelay,
		RetryDelay:   args.RetryDelay,
		LaxAge:       args.LaxAge,
		Tries:        int(args.Tries),

		OutputDirectory: args.Output,
		Username:        username,
		Password:        password,

		Cookies:   cookies,
		Header:    config.MakeHeaders(args.Headers),
		Proxy:     args.Proxy,
		UserAgent: args.UserAgent,
	}

	return scrapeURLs(ctx, cfg, args)
}

func scrapeURLs(ctx context.Context, cfg config.Config, args arguments) error {

	if !ioutil.FileExists(afero.NewOsFs(), cfg.OutputDirectory) {
		db.DeleteFile() // get rid of stale cache
	}

	etagStore := db.Open()
	defer etagStore.Close()

	for _, url := range args.URLs {
		cfg.URL = url
		sc, err := scraper.New(cfg)
		if err != nil {
			return fmt.Errorf("initializing scraper: %w", err)
		}

		sc.ETagsDB = etagStore

		logger.Info("Scraping", slog.String("url", sc.URL.String()))
		if err = sc.Start(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				os.Exit(0)
			}

			return fmt.Errorf("scraping '%s': %w", sc.URL, err)
		}

		if args.SaveCookieFile != "" {
			if err := saveCookies(args.SaveCookieFile, sc.Cookies()); err != nil {
				return fmt.Errorf("saving cookies: %w", err)
			}
		}
	}

	reportHistogram()
	return nil
}

func reportHistogram() {
	m := download.Counters.Map()
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	for _, key := range keys {
		logger.Warn(fmt.Sprintf("%3d: %d", key, m[key]))
	}
}

func runServer(ctx context.Context, args arguments) error {
	if err := scraper.ServeDirectory(ctx, args.Serve, args.ServerPort); err != nil {
		return fmt.Errorf("serving directory: %w", err)
	}
	return nil
}

func createLogger(args arguments) *slog.Logger {
	opts := &slog.HandlerOptions{}

	if args.Debug {
		opts.Level = slog.LevelDebug
	} else if args.Verbose {
		opts.Level = slog.LevelInfo
	} else {
		opts.Level = slog.LevelWarn
	}

	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func readCookieFile(cookieFile string) ([]config.Cookie, error) {
	if cookieFile == "" {
		return nil, nil
	}
	b, err := os.ReadFile(cookieFile)
	if err != nil {
		return nil, fmt.Errorf("reading cookie file: %w", err)
	}

	var cookies []config.Cookie
	if err := json.Unmarshal(b, &cookies); err != nil {
		return nil, fmt.Errorf("unmarshaling cookies: %w", err)
	}

	return cookies, nil
}

func saveCookies(cookieFile string, cookies []config.Cookie) error {
	if cookieFile == "" || len(cookies) == 0 {
		return nil
	}

	b, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("marshaling cookies: %w", err)
	}

	if err := os.WriteFile(cookieFile, b, 0644); err != nil {
		return fmt.Errorf("saving cookies: %w", err)
	}

	return nil
}

// formatVersion builds a version string based on binary release information.
func formatVersion(version, commit, date string) string {
	buf := strings.Builder{}
	buf.WriteString(version)

	if commit != "" {
		buf.WriteString(" commit: " + commit)
	}
	if date != "" {
		buf.WriteString(" built at: " + date)
	}
	goVersion := runtime.Version()
	buf.WriteString(" built with: " + goVersion)
	return buf.String()
}
