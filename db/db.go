package db

import (
	"bufio"
	"fmt"
	"github.com/cornelk/goscrape/logger"
	"github.com/spf13/afero"
	"io"
	"log/slog"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Item struct {
	ETags   string
	Expires time.Time
}

func (i Item) Empty() bool { return len(i.ETags) == 0 && i.Expires.IsZero() }

func (i Item) String() string {
	expires := i.Expires.Format(time.RFC3339)
	if i.Expires.IsZero() {
		expires = "-"
	}
	if len(i.ETags) == 0 {
		return expires
	} else {
		return fmt.Sprintf("%s\t%s", expires, i.ETags)
	}
}

//-------------------------------------------------------------------------------------------------

// DB provides a persistent store for HTTP ETags to reduce network traffic when repeating
// a download session. If the store is unavailable for some reason, its methods are no-ops.
type DB struct {
	file    string
	records map[string]Item
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
		store.records = make(map[string]Item)
	}

	return store
}

func readItem(records map[string]Item, line string) {
	parts := strings.Split(line, "\t")
	if len(parts) >= 2 {
		key := parts[0]
		val1 := parts[1]

		var value Item

		switch len(parts) {
		case 2:
			expires, err := time.Parse(time.RFC3339, val1)
			if err == nil {
				value.Expires = expires
			} else {
				value.ETags = val1
			}

		case 3:
			if val1 != "-" {
				expires, _ := time.Parse(time.RFC3339, val1)
				value.Expires = expires
			}
			value.ETags = parts[2]
		}

		records[key] = value
	}
}

func readFile(rdr io.Reader) (map[string]Item, error) {
	records := make(map[string]Item)
	s := bufio.NewScanner(rdr)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(line, "#") {
			readItem(records, line)
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
func (store *DB) Lookup(u *urlpkg.URL) Item {
	if store == nil {
		return Item{} // no-op if absent
	}

	v := *u
	v.Fragment = ""
	return store.records[v.String()]
}

// Store stores the ETags for a given URL.
func (store *DB) Store(u *urlpkg.URL, item Item) {
	if store == nil {
		return // no-op if absent
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	v := *u
	v.Fragment = ""
	key := v.String()
	if item.Empty() {
		delete(store.records, key)
	} else {
		store.records[key] = item
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
		if err := writeItem(buf, key, store.records[key]); err != nil {
			logger.Warn("Cannot create DB", slog.Any("err", err), slog.String("file", store.file))
			return
		}
	}
	buf.Flush()
}

func writeItem(w io.Writer, key string, value Item) (err error) {
	if value.Empty() {
		return nil
	}
	_, err = fmt.Fprintf(w, "%s\t%s\n", key, value)
	return err
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
