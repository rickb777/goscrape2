package ioutil

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cornelk/goscrape/logger"
	"github.com/spf13/afero"
)

// randomSuffix is appended to files temporarily whilst they are being written
var randomSuffix string

func init() {
	randomSuffix = "." + strconv.FormatInt(rand.Int64N(2^20), 36)
}

// CreateDirectory creates the download path if it does not exist yet.
func CreateDirectory(fs afero.Fs, path string) error {
	if path == "" {
		return nil
	}

	logger.Debug("Creating dir", slog.String("path", path))
	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("creating directory '%s': %w", path, err)
	}
	return nil
}

func WriteFileAtomically(fs afero.Fs, startURL *url.URL, filePath string, data io.Reader) (int64, error) {
	dir := filepath.Dir(filePath)
	if len(dir) < len(startURL.Host) { // nothing to append if it is the root dir
		dir = filepath.Join(".", startURL.Host, dir)
	}

	if err := CreateDirectory(fs, dir); err != nil {
		return 0, err
	}

	logger.Debug("Creating file", slog.String("path", filePath))
	// writing the file may take much time, so write to a temporary file first
	f, err := fs.Create(filePath + randomSuffix)
	if err != nil {
		return 0, fmt.Errorf("creating file '%s': %w", filePath, err)
	}

	var length int64
	if length, err = io.Copy(f, data); err != nil {
		// nolint: wrapcheck
		_ = f.Close() // try to close and remove file but ignore any error
		_ = fs.Remove(filePath + randomSuffix)
		return length, fmt.Errorf("writing to file: %w", err)
	}

	if err := f.Close(); err != nil {
		return length, fmt.Errorf("closing file: %w", err)
	}

	// rename the file so it appears (almost) instantly in the filesystem
	if err := fs.Rename(filePath+randomSuffix, filePath); err != nil {
		return length, fmt.Errorf("renaming %s to %s: %w", filePath+randomSuffix, filePath, err)
	}
	return length, nil
}

//func OpenFile(fs afero.Fs, startURL *url.URL, filePath string) (io.ReadCloser, error) {
//	dir := filepath.Dir(filePath)
//	if len(dir) < len(startURL.Host) { // nothing to append if it is the root dir
//		dir = filepath.Join(".", startURL.Host, dir)
//	}
//
//	f, err := fs.Open(filePath)
//	if err != nil {
//		return nil, fmt.Errorf("reading file '%s': %w", filePath, err)
//	}
//
//	data := &bytes.Buffer{}
//	if _, err = io.Copy(data, f); err != nil {
//		// nolint: wrapcheck
//		_ = f.Close() // try to close and remove file but return the first error
//		return nil, fmt.Errorf("reading from  file: %w", err)
//	}
//
//	return f, nil
//}

func ReadFile(fs afero.Fs, startURL *url.URL, filePath string) ([]byte, error) {
	dir := filepath.Dir(filePath)
	if len(dir) < len(startURL.Host) { // nothing to append if it is the root dir
		dir = filepath.Join(".", startURL.Host, dir)
	}

	f, err := fs.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file '%s': %w", filePath, err)
	}
	defer f.Close()

	data := &bytes.Buffer{}
	if _, err = io.Copy(data, f); err != nil {
		// nolint: wrapcheck
		_ = f.Close() // try to close and remove file but return the first error
		return nil, fmt.Errorf("reading from  file: %w", err)
	}

	return data.Bytes(), nil
}

func FileExists(fs afero.Fs, filePath string) bool {
	_, err := fs.Stat(filePath)
	return !os.IsNotExist(err)
}
