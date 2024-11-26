package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cornelk/goscrape/config"
	"github.com/cornelk/goscrape/db"
	"github.com/cornelk/goscrape/download"
	"github.com/cornelk/goscrape/download/ioutil"
	"github.com/cornelk/goscrape/images"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/scraper"
	"github.com/spf13/afero"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type Strings []string

// String is an implementation of the flag.Value interface
func (i *Strings) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set is an implementation of the flag.Value interface
func (i *Strings) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type Arguments struct {
	URLs []string

	Include Strings
	Exclude Strings
	Output  string

	Concurrency  int
	Depth        int
	ImageQuality int
	Timeout      time.Duration
	LoopDelay    time.Duration
	LaxAge       time.Duration
	Tries        int

	Serve      string
	Origin     string
	ServerPort int

	CookieFile     string
	SaveCookieFile string

	Headers   Strings
	Proxy     string
	User      string
	UserAgent string

	Verbose bool
	Debug   bool
}

func declareFlags() Arguments {
	var arguments Arguments

	flag.Var(&arguments.Include, "i", "only include URLs that match a regular expression (can be repeated)")
	flag.Var(&arguments.Exclude, "x", "exclude URLs that match a regular expression (can be repeated)")
	flag.StringVar(&arguments.Output, "o", "", "output `directory` to write files to")

	flag.IntVar(&arguments.Concurrency, "concurrency", 1, "the number of concurrent downloads")
	flag.IntVar(&arguments.Depth, "depth", 0, "download depth limit, 0 for unlimited")
	flag.IntVar(&arguments.ImageQuality, "imagequality", 0, "image quality reduction, 0 to disable re-encoding, maximum 99")
	flag.DurationVar(&arguments.Timeout, "timeout", 0, "time limit (with units, e.g. 1s) for each HTTP request to connect and read the response")
	flag.DurationVar(&arguments.LoopDelay, "loopdelay", 0, "delay (with units, e.g. 1s) used between any two downloads")
	flag.DurationVar(&arguments.LaxAge, "laxage", 0, "adds to the 'expires' timestamp specified by the origin server, or creates one if absent; if the origin is too conservative, this helps when doing successive runs; a negative value causes revalidation")
	flag.IntVar(&arguments.Tries, "tries", 1, "the number of tries to download each file if the server gives a 5xx error")

	flag.StringVar(&arguments.Serve, "serve", "", "serve the website using a webserver rooted at the specified path; this disables scraping")
	flag.StringVar(&arguments.Origin, "origin", "", "set the origin server used when serving the website (optional)")
	flag.IntVar(&arguments.ServerPort, "port", 8080, "port to use for the webserver")

	flag.StringVar(&arguments.CookieFile, "cookies", "", "file containing the cookie content")
	flag.StringVar(&arguments.SaveCookieFile, "savecookiefile", "", "file to save the cookie content")

	flag.Var(&arguments.Headers, "header", "HTTP header to use for scraping (can be repeated)")
	flag.StringVar(&arguments.Proxy, "proxy", "", "HTTP proxy to use for scraping")
	flag.StringVar(&arguments.User, "user", "", "user[:password] to use for HTTP authentication")
	flag.StringVar(&arguments.UserAgent, "useragent", "", "user agent to use for scraping")

	flag.BoolVar(&arguments.Verbose, "v", false, "verbose output")
	flag.BoolVar(&arguments.Debug, "z", false, "debug output")

	flag.Parse()
	arguments.URLs = flag.Args()

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Scrape a website and create an offline browsable version on the disk.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nVersion %s\n", formatVersion(version, commit, date))
	}
	return arguments
}

func main() {
	args := declareFlags()

	ctx := context.Background()
	//ctx := app.Context() // provides signal handler cancellation

	logger.Logger = createLogger(args)

	if args.Serve != "" {
		if err := runServer(ctx, args); err != nil {
			fmt.Printf("Server execution error: %s\n", err)
			os.Exit(1)
		}
	} else if len(args.URLs) > 0 {
		if err := runScraper(ctx, args); err != nil {
			fmt.Printf("Scraping execution error: %s\n", err)
			os.Exit(1)
		}
	} else {
		flag.Usage()
	}
}

func buildConfig(args Arguments) (*config.Config, error) {
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
		return nil, fmt.Errorf("reading cookie: %w", err)
	}

	cfg := config.Config{
		Includes: args.Include,
		Excludes: args.Exclude,

		Concurrency:  int(args.Concurrency),
		MaxDepth:     int(args.Depth),
		ImageQuality: images.ImageQuality(imageQuality),
		Timeout:      args.Timeout,
		LoopDelay:    args.LoopDelay,
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

	return &cfg, nil
}

func runScraper(ctx context.Context, args Arguments) error {
	//if len(args.URLs) == 0 {
	//	return nil
	//}

	cfg, err := buildConfig(args)
	if cfg == nil || err != nil {
		return err
	}

	fs := afero.NewOsFs()

	if !ioutil.FileExists(fs, cfg.OutputDirectory) {
		db.DeleteFile(fs) // get rid of stale cache
	}

	return scrapeURLs(ctx, fs, *cfg, args.SaveCookieFile, args.URLs)
}

func scrapeURLs(ctx context.Context, fs afero.Fs, cfg config.Config, saveCookieFile string, urls []string) error {
	etagStore := db.Open()
	defer etagStore.Close()

	for _, url := range urls {
		sc, err := scraper.New(cfg, url, fs)
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

		if saveCookieFile != "" {
			if err := saveCookies(saveCookieFile, sc.Cookies()); err != nil {
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

func runServer(ctx context.Context, args Arguments) error {
	var sc *scraper.Scraper

	if args.Origin != "" {
		cfg, err := buildConfig(args)
		if cfg == nil || err != nil {
			return err
		}

		fs := afero.NewOsFs()
		sc, err = scraper.New(*cfg, args.Origin, fs)
		if err == nil {
			return fmt.Errorf("serving directory for %s: %w", args.Origin, err)
		}
	}

	if err := scraper.ServeDirectory(ctx, args.Serve, int16(args.ServerPort), sc); err != nil {
		return fmt.Errorf("serving directory: %w", err)
	}
	return nil
}

func createLogger(args Arguments) *slog.Logger {
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
