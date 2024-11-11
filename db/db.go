package db

import (
	"log/slog"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/cornelk/goscrape/logger"
	"github.com/rickb777/acceptable/header"
)

// DB provides a persistent store for HTTP ETags to reduce network traffic when repeating
// a download session. If the store is unavailable for some reason, its methods are no-ops.
type DB struct {
	db   *bolt.DB
	file string
}

func Open() *DB {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir != "" {
		return OpenDB(dir)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	if home != "" {
		return OpenDB(filepath.Join(home, ".config"))
	}

	logger.Warn("Cannot access ETag database in $XDG_CONFIG_HOME or $HOME/.config")
	return nil // this will hurt performance but downloading will still work
}

const FileName = "goscrape.db"

func OpenDB(dir string) *DB {
	file := filepath.Join(dir, FileName)

	store, err := bolt.Open(file, 0644, nil)
	if err != nil {
		logger.Error("Cannot access ETag database", slog.String("file", "file"))
		return nil
	}
	return &DB{db: store, file: file}
}

func (store *DB) Close() error {
	if store == nil {
		return nil // no-op if absent
	}
	return store.db.Close()
}

// Lookup finds the ETags for a given URL.
func (store *DB) Lookup(u *url.URL) (etags header.ETags) {
	if store == nil {
		return nil // no-op if absent
	}

	err := store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(nonBlankKey(u.Host))
		if b == nil {
			return nil
		}

		parts, filename := splitPath(u.Path)

		for _, part := range parts {
			b = b.Bucket(nonBlankKey(part))
			if b == nil {
				return nil
			}
		}

		value := string(b.Get(nonBlankKey(filename)))
		etags = header.ETagsOf(value)
		return nil
	})
	if err != nil {
		logger.Warn("Cannot view DB", slog.Any("err", err), slog.String("file", store.file))
	}

	return etags
}

// Store stores the ETags for a given URL.
func (store *DB) Store(u *url.URL, etags header.ETags) {
	if store == nil {
		return // no-op if absent
	}

	err := store.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(nonBlankKey(u.Host))
		if err != nil {
			logger.Warn("Cannot create DB bucket", slog.Any("err", err), slog.Any("url", u), slog.String("name", u.Host))
			return nil
		}

		parts, filename := splitPath(u.Path)

		for _, part := range parts {
			b, err = b.CreateBucketIfNotExists(nonBlankKey(part))
			if err != nil {
				logger.Warn("Cannot create DB bucket", slog.Any("err", err), slog.Any("url", u), slog.String("name", part))
				return nil
			}
		}

		err = b.Put(nonBlankKey(filename), []byte(header.ETags(etags).String()))
		if err != nil {
			logger.Warn("Cannot put DB bucket value", slog.Any("err", err), slog.Any("url", u), slog.String("name", parts[len(parts)-1]))
		}

		return nil
	})
	if err != nil {
		logger.Warn("Cannot update DB", slog.Any("err", err))
	}
}

func splitPath(path string) ([]string, string) {
	if strings.HasSuffix(path, "/") {
		parts := strings.Split(pathpkg.Clean(path), "/")[1:]
		return parts, ""
	}
	parts := strings.Split(pathpkg.Clean(path), "/")[1:]
	switch {
	case len(parts) == 0:
		return nil, ""
	case len(parts) == 1:
		return nil, parts[0]
	default:
		j := strings.Join(parts[:len(parts)-1], "/")
		return []string{j}, parts[len(parts)-1]
	}
}

func nonBlankKey(s string) []byte {
	if s == "" {
		return []byte{1} // safe alternative to a string
	}
	return []byte(s)
}
