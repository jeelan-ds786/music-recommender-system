package logger

import (
	"log"
	"os"
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

func (l *Logger) Debug(format string, args ...any) {
	if l.level >= LevelDebug {
		l.out.Printf("[DEBUG] "+format, args...)
	}
}

func (l *Logger) Info(format string, args ...any) {
	if l.level >= LevelInfo {
		l.out.Printf("[INFO] "+format, args...)
	}
}

func (l *Logger) Error(format string, args ...any) {
	if l.level >= LevelError {
		l.out.Printf("[ERROR] "+format, args...)
	}
}
