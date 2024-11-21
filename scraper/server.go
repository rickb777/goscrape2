package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"

	"github.com/cornelk/goscrape/logger"
	"github.com/rickb777/servefiles/v3"
)

// set more mime types in the browser, this for example fixes .asp files not being
// downloaded but handled as html.
var mimeTypes = map[string]string{
	".asp": "text/html; charset=utf-8",
}

type onDemand struct {
	sc *Scraper
}

func (h *onDemand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//h.sc.Downloader()
	http.Error(w, "not implemented", 404)
}

func ServeDirectory(ctx context.Context, path string, port int16, sc *Scraper) error {
	fileServer := servefiles.NewAssetHandlerIoFS(os.DirFS(path))

	//if sc != nil {
	//	fileServer.NotFound = &onDemand{sc: sc}
	//}

	//servefiles.Debugf = func(format string, v ...interface{}) { fmt.Printf(format, v...) }
	mux := http.NewServeMux()
	mux.Handle("/", fileServer)

	// update mime types
	for ext, mt := range mimeTypes {
		if err := mime.AddExtensionType(ext, mt); err != nil {
			return fmt.Errorf("adding mime type '%s': %w", ext, err)
		}
	}

	fullAddr := fmt.Sprintf("http://127.0.0.1:%d", port)
	logger.Info("Serving directory...",
		slog.String("path", path),
		slog.String("address", fullAddr))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
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
		return fmt.Errorf("starting webserver: %w", err)
	}
}
