package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	urlpkg "net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/rickb777/goscrape2/config"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/download"
	"github.com/rickb777/goscrape2/download/ioutil"
	"github.com/rickb777/goscrape2/images"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/scraper"
	"github.com/rickb777/goscrape2/server"
	"github.com/rickb777/servefiles/v3"
	"github.com/spf13/afero"
)

var (
	version = "dev"
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

//-------------------------------------------------------------------------------------------------

type Arguments struct {
	URLs []*urlpkg.URL

	Include   Strings
	Exclude   Strings
	Directory string

	Concurrency  int
	Depth        int
	ImageQuality int
	Timeout      time.Duration
	LoopDelay    time.Duration
	LaxAge       time.Duration
	Tries        int

	Serve      bool
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

	flag.Var(&arguments.Include, "i", "only include URLs that match a `regular expression` (can be repeated)")
	flag.Var(&arguments.Exclude, "x", "exclude URLs that match a `regular expression` (can be repeated)")
	flag.StringVar(&arguments.Directory, "dir", "", "`directory` to write files to and to serve files from")

	flag.IntVar(&arguments.Concurrency, "concurrency", 1, "the number of concurrent downloads")
	flag.IntVar(&arguments.Depth, "depth", 0, "download depth limit (default unlimited)")
	flag.IntVar(&arguments.ImageQuality, "imagequality", 0, "image quality reduction, minimum 1 to maximum 99 (re-encoding disabled by default)")
	flag.DurationVar(&arguments.Timeout, "timeout", 0, "time limit (with units, e.g. 1s) for each HTTP request to connect and read the response")
	flag.DurationVar(&arguments.LoopDelay, "loopdelay", 0, "delay (with units, e.g. 1s) used between any two downloads")
	flag.DurationVar(&arguments.LaxAge, "laxage", 0, "adds to the 'expires' timestamp specified by the origin server, or creates one if absent; if the origin is too conservative, this helps when doing successive runs; a negative value causes revalidation instead")
	flag.IntVar(&arguments.Tries, "tries", 1, "the number of tries to download each file if the server gives a 5xx error")

	flag.BoolVar(&arguments.Serve, "serve", false, "serve the website using a webserver; scraping will happen only on demand using the first URL you provide")
	flag.IntVar(&arguments.ServerPort, "port", 8080, "port to use for the webserver")

	flag.StringVar(&arguments.CookieFile, "cookies", "", "file containing the cookie content")
	flag.StringVar(&arguments.SaveCookieFile, "savecookiefile", "", "file to save the cookie content")

	flag.Var(&arguments.Headers, "H", "\"name:value\" HTTP header to use for scraping (can be repeated)")
	flag.StringVar(&arguments.Proxy, "proxy", "", "HTTP proxy to use for scraping")
	flag.StringVar(&arguments.User, "user", "", "user[:password] to use for HTTP authentication")
	flag.StringVar(&arguments.UserAgent, "useragent", "", "user agent to use for scraping")

	flag.BoolVar(&arguments.Verbose, "v", false, "verbose output")
	flag.BoolVar(&arguments.Debug, "z", false, "debug output")

	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Scrape a website and create an offline browsable version on the disk.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [options] [<url> ...]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nOptions also accept '--'.\nVersion %s\n", formatVersion(version, date))
	}
	return arguments
}

//-------------------------------------------------------------------------------------------------

func main() {
	args := declareFlags()

	createLogger(args)

	var err error
	args.URLs, err = parseAll(flag.Args())
	if err != nil {
		fmt.Printf("Invalid URL: %s\n", err)
		logger.Exit(1)
	}

	ctx := context.Background()
	//ctx := app.Context() // provides signal handler cancellation

	if !args.Serve && len(args.URLs) == 0 {
		fmt.Printf("Must provide -serve or URLs to scrape\n")
		flag.Usage()
		logger.Exit(1)
	}

	cfg, err := buildConfig(args)
	if err != nil {
		fmt.Printf("Config error: %s\n", err)
		logger.Exit(1)
	}

	fs := afero.NewOsFs()

	if !ioutil.FileExists(fs, cfg.Directory) {
		db.DeleteFile(fs) // get rid of stale cache
	}

	if len(args.URLs) > 0 {
		if err := scrapeURLs(ctx, fs, *cfg, args.SaveCookieFile, args.Serve, int16(args.ServerPort), args.URLs); err != nil {
			logger.Error("Scraping execution error", slog.Any("error", err))
		}

	} else if args.Serve {
		if err := server.ServeDirectory(ctx, nil, cfg.Directory, int16(args.ServerPort)); err != nil {
			logger.Error("Server execution error", slog.Any("error", err))
		}
	}
	logger.Exit(0)
}

func parseAll(urls []string) (list []*urlpkg.URL, err error) {
	list = make([]*urlpkg.URL, len(urls))
	for i, url := range urls {
		list[i], err = urlpkg.Parse(url)
		if err != nil {
			return nil, err
		}
	}
	return list, nil
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

	return &config.Config{
		Includes: args.Include,
		Excludes: args.Exclude,

		Concurrency:  args.Concurrency,
		MaxDepth:     args.Depth,
		ImageQuality: images.ImageQuality(imageQuality),
		Timeout:      args.Timeout,
		LoopDelay:    args.LoopDelay,
		LaxAge:       args.LaxAge,
		Tries:        args.Tries,

		Directory: args.Directory,
		Username:  username,
		Password:  password,

		Cookies:   cookies,
		Header:    config.MakeHeaders(args.Headers),
		Proxy:     args.Proxy,
		UserAgent: args.UserAgent,
	}, nil
}

func scrapeURLs(ctx context.Context, fs afero.Fs, cfg config.Config, saveCookieFile string, serve bool, serverPort int16, urls []*urlpkg.URL) error {
	etagStore := db.Open()
	defer etagStore.Close()

	var webServer *http.Server
	var errChan chan error

	for i, url := range urls {
		sc, err := scraper.New(cfg, url, afero.NewBasePathFs(fs, cfg.Directory))
		if err != nil {
			return fmt.Errorf("initializing scraper: %w", err)
		}

		sc.ETagsDB = etagStore

		if serve && i == 0 {
			webServer, errChan, err = server.LaunchWebserver(sc, cfg.Directory, serverPort)
			if err != nil {
				return fmt.Errorf("launching webserver: %w", err)
			}
		}

		logger.Info("Scraping", slog.String("url", sc.URL.String()))
		if err = sc.Start(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Exit(1)
			}

			var ue *urlpkg.Error
			if errors.As(err, &ue) {
				if serve {
					logger.Warn("HTTP request failed",
						slog.String("url", url.String()),
						slog.Any("error", err))
					continue // ignore because the webserver is operational
				}
			}

			return fmt.Errorf("HTTP get %s failed: %w", url, err)
		}

		if saveCookieFile != "" {
			if err := saveCookies(saveCookieFile, sc.Cookies()); err != nil {
				return fmt.Errorf("saving cookies url=%s: %w", url, err)
			}
		}
	}

	reportHistogram()

	return server.AwaitWebserver(ctx, webServer, errChan)
}

func reportHistogram() {
	m := download.Counters.Map()
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	logger.Warn("Scraping finished", slog.Int("response-codes", len(keys)))
	for _, key := range keys {
		logger.Warn(fmt.Sprintf("%3d: %d", key, m[key]))
	}
}

func createLogger(args Arguments) {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}

	if args.Debug {
		opts.Level = slog.LevelDebug
		servefiles.Debugf = func(format string, v ...interface{}) { logger.Debug(fmt.Sprintf(format, v...)) }
	} else if args.Verbose {
		opts.Level = slog.LevelInfo
	} else {
		opts.Level = slog.LevelWarn
	}

	logger.Create(os.Stdout, opts)
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
func formatVersion(version, date string) string {
	buf := strings.Builder{}
	buf.WriteString(version)

	buf.WriteString(" built with ")
	buf.WriteString(runtime.Version())
	if date != "" {
		buf.WriteString(" on ")
		buf.WriteString(date)
	}
	buf.WriteString(".")
	return buf.String()
}
