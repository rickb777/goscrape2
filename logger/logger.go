// Package logger is an adaptation wrapper that simplifies logging in the main
// code whilst also allowing a pluggable test logger.
package logger

import "github.com/cornelk/gotokit/log"

// Logger is a global logger that is able to handle concurrent logging safely.
var Logger *log.Logger

func Debug(msg string, args ...any) {
	Logger.Log(nil, log.DebugLevel, msg, args...)
}

func Info(msg string, args ...any) {
	Logger.Log(nil, log.InfoLevel, msg, args...)
}

func Warn(msg string, args ...any) {
	Logger.Log(nil, log.WarnLevel, msg, args...)
}

func Error(msg string, args ...any) {
	Logger.Log(nil, log.ErrorLevel, msg, args...)
}
