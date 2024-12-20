// Package logger is an adaptation wrapper that simplifies logging in the main
// code whilst also allowing a pluggable test logger.
package logger

import (
	"fmt"
	"github.com/rickb777/logrotate"
	sloghttp "github.com/samber/slog-http"
	"log/slog"
	"os"
	"sync/atomic"
)

// Create updates Logger to use a specific log file (or stdout), based
// on a specified log file name.
func Create(logFile string, opts *slog.HandlerOptions) {
	logWriter := logrotate.MustLogWriterWithSignals(logFile, os.Stdout)
	Logger = slog.New(slog.NewTextHandler(logWriter, opts))
}

// HttpLogConfig provides configuration options for the HTTP logger, if used.
func HttpLogConfig() sloghttp.Config {
	for _, hdr := range []string{
		"connection", "dnt", "sec-gpc", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent",
	} {
		sloghttp.HiddenRequestHeaders[hdr] = struct{}{}
	}

	return sloghttp.Config{
		DefaultLevel:       slog.LevelInfo,
		ClientErrorLevel:   slog.LevelWarn,
		ServerErrorLevel:   slog.LevelError,
		WithRequestID:      false,
		WithRequestHeader:  true,
		WithResponseHeader: false,
	}
}

var errorCount atomic.Int64

// Logger is a global logger that is able to handle concurrent logging safely.
var Logger = slog.Default()

func Log(level slog.Level, msg string, args ...any) {
	Logger.Log(nil, level, msg, args...)
}

func Debug(msg string, args ...any) {
	Log(slog.LevelDebug, msg, args...)
}

func Info(msg string, args ...any) {
	Log(slog.LevelInfo, msg, args...)
}

func Warn(msg string, args ...any) {
	Log(slog.LevelWarn, msg, args...)
}

func Error(msg string, args ...any) {
	Log(slog.LevelError, msg, args...)
	errorCount.Add(1)
}

func Errorf(msg string, args ...any) {
	Log(slog.LevelError, fmt.Sprintf(msg, args...))
	errorCount.Add(1)
}

var Exit = func(code int) {
	os.Exit(code + int(errorCount.Load()))
}
