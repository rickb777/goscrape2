package ioutil

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"

	"github.com/rickb777/goscrape2/logger"
	pathpkg "github.com/rickb777/path"
	"github.com/spf13/afero"
)

// randomSuffix is appended to files temporarily whilst they are being written
var randomSuffix pathpkg.Path

func init() {
	randomSuffix = pathpkg.Path("." + strconv.FormatInt(rand.Int64N(2^20), 36))
}

// CreateDirectory creates the download path if it does not exist yet.
func CreateDirectory(fs afero.Fs, path pathpkg.Path) error {
	if path == "" {
		return nil
	}

	logger.Debug("Creating dir", slog.String("path", string(path)))
	if err := fs.MkdirAll(string(path), os.ModePerm); err != nil {
		return fmt.Errorf("creating directory '%s': %w", path, err)
	}
	return nil
}

func WriteFileAtomically(fs afero.Fs, filePath pathpkg.Path, data io.Reader) (int64, error) {
	dir := filePath.Dir()

	if err := CreateDirectory(fs, dir); err != nil {
		return 0, err
	}

	logger.Debug("Creating file", slog.String("path", string(filePath)))
	// writing the file may take much time, so write to a temporary file first
	tempPath := filePath + randomSuffix
	f, err := fs.Create(string(tempPath))
	if err != nil {
		return 0, fmt.Errorf("creating file '%s': %w", filePath, err)
	}

	var length int64
	if length, err = io.Copy(f, data); err != nil {
		// nolint: wrapcheck
		_ = f.Close() // try to close and remove file but ignore any error
		_ = fs.Remove(string(tempPath))
		return length, fmt.Errorf("writing to file: %w", err)
	}

	if err := f.Close(); err != nil {
		return length, fmt.Errorf("closing file: %w", err)
	}

	// rename the file so it appears (almost) instantly in the filesystem
	if err := fs.Rename(string(tempPath), string(filePath)); err != nil {
		return length, fmt.Errorf("renaming %s to %s: %w", tempPath, filePath, err)
	}
	return length, nil
}

func ReadFile(fs afero.Fs, filePath pathpkg.Path) ([]byte, error) {
	f, err := fs.Open(string(filePath))
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

func FileExists(fs afero.Fs, filePath pathpkg.Path) bool {
	_, err := fs.Stat(string(filePath))
	return !os.IsNotExist(err)
}
