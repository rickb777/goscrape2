package scraper

import (
	"context"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/goscrape/work"
	"github.com/rickb777/servefiles/v3"
	"github.com/spf13/afero"
	"log/slog"
	"mime"
	"net/http"
)

// set more mime types in the browser, this for example fixes .asp files not being
// downloaded but handled as html.
var mimeTypes = map[string]string{
	".asp": "text/html; charset=utf-8",
}

type onDemand struct {
	sc *Scraper
	fs afero.Fs
}

func (h *onDemand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := h.sc.URL.ResolveReference(r.URL)
	d := h.sc.Downloader()
	_, result, err := d.ProcessURL(r.Context(), work.Item{URL: url})
	if err != nil {
		http.Error(w, "Bad gateway: "+err.Error(), http.StatusBadGateway)
	} else if result.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("Internal server error: upstream %d", result.StatusCode), http.StatusInternalServerError)
	}
}

func ServeDirectory(ctx context.Context, path string, port int16, sc *Scraper) error {
	fs := afero.NewBasePathFs(afero.NewOsFs(), path)

	fullAddr := fmt.Sprintf("http://127.0.0.1:%d", port)
	logger.Info("Serving directory...",
		slog.String("path", path),
		slog.String("address", fullAddr))

	server, err := newWebserver(fs, port, sc)
	if err != nil {
		return err
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		//nolint: contextcheck
		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutting down webserver: %w", err)
		}
		return nil

	case err := <-serverErr:
		return fmt.Errorf("webserver: %w", err)
	}
}

func newWebserver(fs afero.Fs, port int16, sc *Scraper) (*http.Server, error) {
	//servefiles.Debugf = func(format string, v ...interface{}) { fmt.Printf(format, v...) }
	fileServer := servefiles.NewAssetHandlerFS(fs)

	if sc != nil {
		fileServer.NotFound = &onDemand{sc: sc, fs: fs}
	}

	mux := http.NewServeMux()
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", port)
	return &http.Server{Addr: addr, Handler: mux}, nil
}

func addMoreMIMETypes() {
	for ext, mt := range mimeTypes {
		if err := mime.AddExtensionType(ext, mt); err != nil {
			panic(fmt.Errorf("adding mime type '%s': %w", ext, err))
		}
	}
}

func init() { addMoreMIMETypes() }
