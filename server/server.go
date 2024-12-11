package server

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/rickb777/acceptable/headername"
	"github.com/rickb777/goscrape2/db"
	"github.com/rickb777/goscrape2/logger"
	"github.com/rickb777/goscrape2/scraper"
	"github.com/rickb777/goscrape2/work"
	"github.com/rickb777/servefiles/v3"
	sloghttp "github.com/samber/slog-http"
	"github.com/spf13/afero"
)

const (
	minRedirectCode = http.StatusMovedPermanently
	maxRedirectCode = http.StatusPermanentRedirect
)

// set more mime types in the browser, this fixes .asp files not being
// downloaded but handled as html.
var mimeTypes = map[string]string{
	".asp": "text/html; charset=utf-8",
}

//-------------------------------------------------------------------------------------------------

// onDemand is a HTTP handler that processes 404-not found requests by trying to download
// the missing items from the origin server.
type onDemand struct {
	sc   *scraper.Scraper
	next http.Handler
}

func (h *onDemand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := h.sc.URL.ResolveReference(r.URL)
	d := h.sc.Downloader()
	_, result, err := d.ProcessURL(r.Context(), work.Item{URL: url, Depth: 1})

	if err != nil {
		http.Error(w, "Bad gateway: "+err.Error(), http.StatusBadGateway)

	} else if result.StatusCode == http.StatusNotFound {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

	} else if minRedirectCode <= result.StatusCode && result.StatusCode <= maxRedirectCode {
		// store so that the webserver can replay the redirect
		d.ETagsDB.Store(url, db.Item{Code: result.StatusCode, Location: result.Location})
		w.Header().Set(headername.Location, result.Location)
		w.WriteHeader(result.StatusCode)

	} else if result.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("Internal server error: upstream %d", result.StatusCode), http.StatusInternalServerError)

	} else {
		h.next.ServeHTTP(w, r) // pass this request on to the file server
	}
}

//-------------------------------------------------------------------------------------------------

// redirecter uses the cache database to decide which URLs to redirect and where to redirect those
// URls. Everything else is handled by the next handler.
type redirecter struct {
	eTagsDB *db.DB
	scheme  string
	host    string
	next    http.Handler
}

func (h *redirecter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := *r.URL
	u.Scheme = h.scheme
	u.Host = h.host
	metadata := h.eTagsDB.Lookup(&u)
	if minRedirectCode <= metadata.Code && metadata.Code <= maxRedirectCode && metadata.Location != "" {
		w.Header().Set(headername.Location, metadata.Location)
		w.WriteHeader(metadata.Code)
	} else {
		h.next.ServeHTTP(w, r)
	}
}

//-------------------------------------------------------------------------------------------------

func ServeDirectory(ctx context.Context, sc *scraper.Scraper, path string, port int16) error {
	server, errChan, err := LaunchWebserver(sc, path, port)
	if err != nil {
		return err
	}

	return AwaitWebserver(ctx, server, errChan)
}

func LaunchWebserver(sc *scraper.Scraper, path string, port int16) (*http.Server, chan error, error) {
	logger.Info("Serving directory",
		slog.String("path", path),
		slog.String("address", fmt.Sprintf("http://%s:%d", hostname(), port)))

	handler := constructAssetServer(sc, path)
	handler = &redirecter{eTagsDB: sc.ETagsDB, scheme: sc.URL.Scheme, host: sc.URL.Host, next: handler}
	handler = sloghttp.NewWithConfig(logger.Logger, logger.HttpLogConfig())(handler)
	handler = handlers.RecoveryHandler()(handler)
	server := newWebserver(port, handler)

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()
	return server, errChan, nil
}

func AwaitWebserver(ctx context.Context, server *http.Server, errChan chan error) error {
	if server == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutting down webserver: %w", err)
		}
		return nil

	case err := <-errChan:
		return fmt.Errorf("webserver: %w", err)
	}
}

func newWebserver(port int16, fileServer http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", port)
	return &http.Server{Addr: addr, Handler: mux}
}

func constructAssetServer(sc *scraper.Scraper, path string) http.Handler {
	var fileServer http.Handler
	if sc == nil {
		fs := afero.NewBasePathFs(afero.NewOsFs(), path)
		fileServer = servefiles.NewAssetHandlerFS(fs)
	} else {
		fileServer = assetHandlerWith404Handler(sc)
	}
	return fileServer
}

func assetHandlerWith404Handler(sc *scraper.Scraper) http.Handler {
	fs := afero.NewBasePathFs(sc.Fs, sc.URL.Host)
	secondary := servefiles.NewAssetHandlerFS(fs) // secondary has default 404 handler
	primary := servefiles.NewAssetHandlerFS(fs)
	primary.NotFound = &onDemand{sc: sc, next: secondary}
	return primary
}

func hostname() string {
	hostname := "localhost"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}
	return hostname
}

func addMoreMIMETypes() {
	for ext, mt := range mimeTypes {
		if err := mime.AddExtensionType(ext, mt); err != nil {
			panic(fmt.Errorf("adding mime type '%s': %w", ext, err))
		}
	}
}

func init() { addMoreMIMETypes() }
