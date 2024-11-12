package db

import (
	"bufio"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"github.com/rickb777/acceptable/header"
	"github.com/spf13/afero"
	"io"
	"log/slog"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// DB provides a persistent store for HTTP ETags to reduce network traffic when repeating
// a download session. If the store is unavailable for some reason, its methods are no-ops.
type DB struct {
	file    string
	records map[string]string
	count   int
	fs      afero.Fs
	mu      sync.Mutex
}

func DeleteFile() {
	_ = os.Remove(filepath.Join(configDir(), FileName))
}

func Open() *DB {
	return OpenDB(configDir(), afero.NewOsFs())
}

const FileName = "goscrape-etags.txt"

func OpenDB(dir string, fs afero.Fs) *DB {
	if !fileExists(fs, dir) {
		return nil
	}

	file := filepath.Join(dir, FileName)
	store := &DB{file: file, fs: fs}

	f, err := fs.Open(file)
	if err == nil {
		store.records, err = readFile(f)
	} else {
		store.records = make(map[string]string)
	}

	return store
}

func readFile(rdr io.Reader) (map[string]string, error) {
	records := make(map[string]string)
	s := bufio.NewScanner(rdr)
	for s.Scan() {
		line := s.Text()
		before, after, found := strings.Cut(line, "\t")
		if found && len(before) > 0 && len(after) > 0 {
			records[before] = after
		}
	}
	return records, nil
}

func configDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	if home != "" {
		return filepath.Join(home, ".config")
	}

	return ""
}

func (store *DB) Close() error {
	if store == nil {
		return nil // no-op if absent
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.flush()
	return nil
}

// Lookup finds the ETags for a given URL.
func (store *DB) Lookup(u *urlpkg.URL) header.ETags {
	if store == nil {
		return nil // no-op if absent
	}

	v := *u
	v.Fragment = ""
	etags := store.records[v.String()]
	return header.ETagsOf(etags)
}

func write(w io.Writer, key, value string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", key, value)
	return err
}

// Store stores the ETags for a given URL.
func (store *DB) Store(u *urlpkg.URL, etags header.ETags) {
	if store == nil {
		return // no-op if absent
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	v := *u
	v.Fragment = ""
	key := v.String()
	value := etags.String()
	if value != "" {
		store.records[key] = value
	} else {
		delete(store.records, key)
	}
	store.syncPeriodically()
}

func (store *DB) flush() {
	file, err := store.fs.Create(store.file)
	if err != nil {
		logger.Warn("Cannot create DB", slog.Any("err", err), slog.String("file", store.file))
		return
	}
	defer file.Close()

	keys := make([]string, 0, len(store.records))
	for key := range store.records {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	buf := bufio.NewWriter(file)
	for _, key := range keys {
		if err := write(buf, key, store.records[key]); err != nil {
			logger.Warn("Cannot create DB", slog.Any("err", err), slog.String("file", store.file))
			return
		}
	}
	buf.Flush()
}

// numberOfStoresPerSync balances the cost of writing to disk against the lost stores that
// could happen when the whole app is interrupted.
const numberOfStoresPerSync = 20

// syncPeriodically flushes changes to the disk but only after several store operations have happened.
// The mutex must be already locked.
func (store *DB) syncPeriodically() {
	store.count++
	if store.count >= numberOfStoresPerSync {
		store.count = 0
		store.flush()
	}
}

func fileExists(fs afero.Fs, filePath string) bool {
	_, err := fs.Stat(filePath)
	return !os.IsNotExist(err)
}
