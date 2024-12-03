// Package logger is an adaptation wrapper that simplifies logging in the main
// code whilst also allowing a pluggable test logger.
package logger

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
)

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

var Exit = func() {
	os.Exit(int(errorCount.Load()))
}
