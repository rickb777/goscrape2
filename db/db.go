package db

import (
	"bufio"
	"fmt"
	"github.com/rickb777/acceptable/header"
	"github.com/rickb777/goscrape2/logger"
	"github.com/spf13/afero"
	"io"
	"log/slog"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Item struct {
	Code     int
	Location string // redirection
	Content  header.ContentType
	ETags    string
	Expires  time.Time
}

func (i Item) EmptyContentType() bool {
	return (i.Content.Type == "" && i.Content.Subtype == "") ||
		(i.Content.Type == "*" && i.Content.Subtype == "*")
}

func (i Item) Empty() bool {
	return i.Location == "" && i.EmptyContentType() && len(i.ETags) == 0 && i.Expires.IsZero()
}

func dashIfBlank(s string) string {
	if s == "" {
		s = "-"
	}
	return s
}

func (i Item) String() string {
	ct := "-"
	if i.Content.Type != "" {
		ct = i.Content.String()
	}

	expires := "-"
	if !i.Expires.IsZero() {
		expires = i.Expires.Format(time.RFC3339)
	}

	return fmt.Sprintf("%d\t%s\t%s\t%s\t%s",
		i.Code,
		dashIfBlank(i.Location),
		ct,
		expires,
		dashIfBlank(i.ETags))
}

func parseItem(line string) (string, Item) {
	parts := strings.Split(line, "\t")

	if len(parts) != 6 {
		return "", Item{}
	}

	key := parts[0]
	v1, _ := strconv.Atoi(parts[1])
	v2 := parts[2]
	v3 := parts[3]
	v4 := parts[4]
	v5 := parts[5]

	var ct header.ContentType
	if v3 != "-" {
		ct = header.ParseContentType(v3)
	}

	var expires time.Time
	if v4 != "-" {
		// time.Parse conveniently returns the zero value on error
		expires, _ = time.Parse(time.RFC3339, v4)
	}

	return key, Item{
		Code:     v1,
		Location: strNotDash(v2),
		Content:  ct,
		Expires:  expires,
		ETags:    strNotDash(v5),
	}

}

//-------------------------------------------------------------------------------------------------

// DB provides a persistent store for HTTP ETags and other metadata to reduce network traffic when
// repeating a download session. If the store is unavailable for some reason, its methods are no-ops.
type DB struct {
	file    string
	records map[string]Item
	count   int
	fs      afero.Fs
	mu      sync.Mutex
}

func DeleteFile(fs afero.Fs) {
	_ = fs.Remove(filepath.Join(localStateDir(), FileName))
}

func Open() *DB {
	return OpenDB(localStateDir(), afero.NewOsFs())
}

const FileName = "goscrape-cache.txt"

func OpenDB(dir string, fs afero.Fs) *DB {
	if err := fs.MkdirAll(dir, 0755); err != nil {
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

func strNotDash(s string) string {
	if s == "-" {
		s = ""
	}
	return s
}

func readFile(rdr io.Reader) (map[string]Item, error) {
	records := make(map[string]Item)
	s := bufio.NewScanner(rdr)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(line, "#") {
			key, item := parseItem(line)
			if key != "" {
				records[key] = item
			}
		}
	}
	return records, nil
}

// localStateDir gets the XDG state directory, a place for storage of volatile
// application state. See https://specifications.freedesktop.org/basedir-spec/
func localStateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	if home != "" {
		return filepath.Join(home, ".local/state")
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

func keyOf(u *urlpkg.URL) string {
	v := *u
	v.Fragment = ""
	return v.String()
}

// Lookup finds the metadata for a given URL.
func (store *DB) Lookup(u *urlpkg.URL) Item {
	if store == nil {
		return Item{} // no-op if absent
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	return store.records[keyOf(u)]
}

// Store stores the metadata for a given URL.
func (store *DB) Store(u *urlpkg.URL, item Item) {
	if store == nil {
		return // no-op if absent
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if item.Empty() {
		delete(store.records, keyOf(u))
	} else {
		store.records[keyOf(u)] = item
	}

	store.syncPeriodically()
}

func (store *DB) flush() {
	// store.mu is already locked

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

	// the keys are sorted - not strictly necessary but it aids manual inspection
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

func writeItem(w io.Writer, key string, item Item) (err error) {
	if item.Empty() {
		return nil
	}
	_, err = fmt.Fprintf(w, "%s\t%s\n", key, item)
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
