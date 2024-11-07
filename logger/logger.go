// Package logger is an adaptation wrapper that simplifies logging in the main
// code whilst also allowing a pluggable test logger.
package logger

import "github.com/cornelk/gotokit/log"

// Logger is a global logger that is able to handle concurrent logging safely.
var Logger *log.Logger

func Log(level log.Level, msg string, args ...any) {
	Logger.Log(nil, level, msg, args...)
}

func Debug(msg string, args ...any) {
	Log(log.DebugLevel, msg, args...)
}

func Info(msg string, args ...any) {
	Log(log.InfoLevel, msg, args...)
}

func Warn(msg string, args ...any) {
	Log(log.WarnLevel, msg, args...)
}

func Error(msg string, args ...any) {
	Log(log.ErrorLevel, msg, args...)
}
