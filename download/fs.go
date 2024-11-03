package download

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/cornelk/goscrape/logger"
	"github.com/cornelk/gotokit/log"
)

// CreateDirectory creates the download path if it does not exist yet.
var CreateDirectory = func(path string) error {
	if path == "" {
		return nil
	}

	logger.Debug("Creating dir", log.String("path", path))
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("creating directory '%s': %w", path, err)
	}
	return nil
}

var WriteFile = func(startURL *url.URL, filePath string, data io.Reader) error {
	dir := filepath.Dir(filePath)
	if len(dir) < len(startURL.Host) { // nothing to append if it is the root dir
		dir = filepath.Join(".", startURL.Host, dir)
	}

	if err := CreateDirectory(dir); err != nil {
		return err
	}

	logger.Debug("Creating file", log.String("path", filePath))
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file '%s': %w", filePath, err)
	}

	if _, err = io.Copy(f, data); err != nil {
		// nolint: wrapcheck
		_ = f.Close() // try to close and remove file but return the first error
		_ = os.Remove(filePath)
		return fmt.Errorf("writing to file: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}
	return nil
}

var FileExists = func(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
