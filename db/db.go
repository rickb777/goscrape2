package db

import (
	"bufio"
	"fmt"
	"github.com/rickb777/goscrape2/logger"
	"github.com/spf13/afero"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const FileName = "goscrape-cache.txt"

// DB provides a persistent store for HTTP ETags and other metadata to reduce network traffic when
// repeating a download session. If the store is unavailable for some reason, its methods are no-ops.
type DB struct {
	dir          string
	records      map[string]Item
	unsavedItems int
	fs           afero.Fs
	mu           sync.Mutex
}

func DeleteFile(fs afero.Fs) {
	_ = fs.Remove(filepath.Join(localStateDir(), FileName))
}

func Open() *DB {
	return OpenDB(localStateDir(), afero.NewOsFs())
}

func OpenDB(dir string, fs afero.Fs) *DB {
	dir = filepath.Clean(dir)

	if err := fs.MkdirAll(dir, 0755); err != nil {
		return nil
	}

	store := &DB{dir: appendSlash(dir), fs: fs, records: make(map[string]Item)}

	fileName := filepath.Join(dir, FileName)
	f, err := fs.Open(fileName)
	if err == nil {
		defer f.Close()
		store.records, err = readFile(f)
	}

	go store.syncPeriodically(time.Second)
	return store
}

func appendSlash(dir string) string {
	separator := string([]rune{filepath.Separator})
	if strings.HasSuffix(dir, separator) {
		return dir
	}
	return dir + separator
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

	if store.unsavedItems > 0 {
		store.writeFileAtomically()
	}

	store.dir = "" // marks this store as being closed
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
func (store *DB) Store(url *urlpkg.URL, item Item) {
	if store == nil {
		return // no-op if absent
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	switch item.Code {
	case http.StatusOK, http.StatusNotFound:
		store.records[keyOf(url)] = item

	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		if item.Location != "" {
			store.records[keyOf(url)] = item
		} else {
			delete(store.records, keyOf(url))
		}

	default:
		if item.Empty() {
			delete(store.records, keyOf(url))
		} else {
			panic(fmt.Sprintf("%s %+v", url, item))
		}
	}

	store.unsavedItems++
	if store.unsavedItems >= maxNumberOfUnsavedItems {
		store.writeFileAtomically()
	}
}

// maxNumberOfUnsavedItems balances the cost of writing to disk against the lost items that
// could happen when the whole app is interrupted.
const maxNumberOfUnsavedItems = 100

// writeFileAtomically writes all the items to a specified file via a temporary file
// to give an atomic write in the filesystem.
// The mutex must be already locked.
func (store *DB) writeFileAtomically() {
	if store.dir == "" {
		return // already closed
	}

	temporaryName := store.dir + randomName()
	store.writeFile(temporaryName)

	// rename the file so it appears (almost) instantly in the filesystem
	//_ = os.Remove(store.file) -- not needed on Linux
	if err := store.fs.Rename(temporaryName, store.dir+FileName); err != nil {
		_ = store.fs.Remove(temporaryName)
		logger.Warn("Cannot rename DB", slog.Any("temp", temporaryName), slog.String("file", store.dir+FileName))
	} else {
		logger.Debug("Wrote DB", slog.String("file", store.dir+FileName))
	}

	store.unsavedItems = 0
}

// writeFile writes all the items to a specified file. The URLs are sorted alphabetically.
// The mutex must be already locked.
func (store *DB) writeFile(fileName string) {
	file, err := store.fs.Create(fileName)
	if err != nil {
		logger.Warn("Cannot create DB", slog.Any("err", err), slog.String("file", fileName))
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
			logger.Warn("Cannot write DB", slog.Any("err", err), slog.String("file", fileName))
			return
		}
	}
	buf.Flush()
}

func writeItem(w io.Writer, key string, item Item) (err error) {
	if item.Empty() {
		return nil
	}
	ss := make([]string, 1, 6)
	ss[0] = key
	ss = append(ss, item.Strings()...)
	_, err = fmt.Fprintln(w, strings.Join(ss, "\t"))
	return err
}

// syncPeriodically is run as a goroutine to write the file periodically when there are changes,
// stopping when Close has been called.
func (store *DB) syncPeriodically(delay time.Duration) {
	busy := true
	for busy {
		time.Sleep(delay)
		store.mu.Lock()

		busy = store.dir != ""
		if busy && store.unsavedItems > 0 {
			store.writeFileAtomically()
		}

		store.mu.Unlock()
	}
}

func randomName() string {
	return "." + strconv.FormatUint(uint64(rand.Uint32()), 36)
}
