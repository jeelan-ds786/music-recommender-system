package logger

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Level int

const (
	LevelNone Level = iota
	LevelError
	LevelInfo
	LevelDebug
)

// ParseLevel reads LOG_LEVEL values: "debug", "info", "error", "none"/"off".
// Unrecognized or empty values default to LevelInfo.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "error":
		return LevelError
	case "none", "off":
		return LevelNone
	default:
		return LevelInfo
	}
}

type Logger struct {
	level Level
	out   *log.Logger
}

func New(level Level) *Logger {
	return &Logger{
		level: level,
		out:   log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Debug/Info/Error each tag the line with <requestID>.<file>.<method>, where
// file and method are the caller's own — captured automatically via
// runtime.Caller so call sites never have to pass them. rid is whatever the
// caller has on hand (typically reqid.FromContext(ctx)); pass "" for
// non-request-scoped logs (e.g. startup), rendered as "-".
func (l *Logger) Debug(rid string, format string, args ...any) {
	l.logAt(LevelDebug, "DEBUG", rid, format, args...)
}

func (l *Logger) Info(rid string, format string, args ...any) {
	l.logAt(LevelInfo, "INFO", rid, format, args...)
}

func (l *Logger) Error(rid string, format string, args ...any) {
	l.logAt(LevelError, "ERROR", rid, format, args...)
}

func (l *Logger) logAt(level Level, tag string, rid string, format string, args ...any) {
	if l.level < level {
		return
	}

	file, method := callerInfo()

	if rid == "" {
		rid = "-"
	}

	prefix := "[" + tag + "] [" + rid + "." + file + "." + method + "] "
	l.out.Printf(prefix+format, args...)
}

// callerInfo walks up to the frame that called Debug/Info/Error (skipping
// logAt and callerInfo itself) and returns its base filename and function
// name, e.g. "service.go", "GetMe".
func callerInfo() (file, method string) {
	pc, fullPath, _, ok := runtime.Caller(3)
	if !ok {
		return "unknown", "unknown"
	}

	file = filepath.Base(fullPath)

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return file, "unknown"
	}

	name := fn.Name()
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	return file, name
}
