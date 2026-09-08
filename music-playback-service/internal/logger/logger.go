package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	log   *zap.Logger
}

type Field = zap.Field

func New(level Level) *Logger {
	return NewWithWriter(level, os.Stdout)
}

func NewWithWriter(level Level, output io.Writer) *Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.MessageKey = "message"

	return &Logger{
		level: level,
		log: zap.New(
			zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(output), zapcore.DebugLevel),
			zap.AddCaller(),
			zap.AddCallerSkip(2),
		),
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

func (l *Logger) InfoFields(rid, message string, fields ...zap.Field) {
	l.logFields(LevelInfo, zapcore.InfoLevel, rid, message, fields...)
}

func String(key, value string) zap.Field                 { return zap.String(key, value) }
func Int(key string, value int) zap.Field                { return zap.Int(key, value) }
func Duration(key string, value time.Duration) zap.Field { return zap.Duration(key, value) }

func (l *Logger) logAt(level Level, tag string, rid string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)

	var zapLevel zapcore.Level
	switch level {
	case LevelError:
		zapLevel = zapcore.ErrorLevel
	case LevelInfo:
		zapLevel = zapcore.InfoLevel
	default:
		zapLevel = zapcore.DebugLevel
	}

	l.logFields(level, zapLevel, rid, message, zap.String("legacy_level", strings.ToLower(tag)))
}

func (l *Logger) logFields(level Level, zapLevel zapcore.Level, rid, message string, fields ...zap.Field) {
	if l.level < level {
		return
	}

	if rid == "" {
		rid = "-"
	}

	fields = append([]zap.Field{zap.String("request_id", rid)}, fields...)
	if checked := l.log.Check(zapLevel, message); checked != nil {
		checked.Write(fields...)
	}
}
