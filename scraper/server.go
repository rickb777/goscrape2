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

// set more mime types in the browser, this fixes .asp files not being
// downloaded but handled as html.
var mimeTypes = map[string]string{
	".asp": "text/html; charset=utf-8",
}

type onDemand struct {
	sc         *Scraper
	fileServer *servefiles.Assets
}

func (h *onDemand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := h.sc.URL.ResolveReference(r.URL)
	d := h.sc.Downloader()
	_, result, err := d.ProcessURL(r.Context(), work.Item{URL: url, Depth: 1})

	if err != nil {
		http.Error(w, "Bad gateway: "+err.Error(), http.StatusBadGateway)
	} else if result.StatusCode == http.StatusNotFound {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	} else if result.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("Internal server error: upstream %d", result.StatusCode), http.StatusInternalServerError)
	} else {
		h.fileServer.ServeHTTP(w, r)
	}
}

//-------------------------------------------------------------------------------------------------

func ServeDirectory(ctx context.Context, path string, port int16, sc *Scraper) error {
	var fs afero.Fs
	if sc == nil {
		fs = afero.NewBasePathFs(afero.NewOsFs(), path)
	}

	logger.Info("Serving directory",
		slog.String("path", path),
		slog.String("address", fmt.Sprintf("http://localhost:%d", port)))

	server, err := newWebserver(port, fs, sc)
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

func newWebserver(port int16, fs afero.Fs, sc *Scraper) (*http.Server, error) {
	var fileServer *servefiles.Assets
	if sc == nil {
		fileServer = servefiles.NewAssetHandlerFS(fs)
	} else {
		fs := afero.NewBasePathFs(sc.fs, sc.URL.Host)
		fileServer = servefiles.NewAssetHandlerFS(fs)
		secondary := servefiles.NewAssetHandlerFS(fs) // secondary has default 404 handler
		fileServer.NotFound = &onDemand{sc: sc, fileServer: secondary}
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
