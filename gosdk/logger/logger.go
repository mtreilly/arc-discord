package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Level represents a log level
type Level int

const (
	// DebugLevel for debug messages
	DebugLevel Level = iota
	// InfoLevel for informational messages
	InfoLevel
	// WarnLevel for warning messages
	WarnLevel
	// ErrorLevel for error messages
	ErrorLevel
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

// Logger represents a structured logger
type Logger struct {
	level  Level
	format string // "json" or "text"
	writer io.Writer
}

// New creates a new logger
func New(level Level, format string, writer io.Writer) *Logger {
	if writer == nil {
		writer = os.Stderr
	}
	return &Logger{
		level:  level,
		format: format,
		writer: writer,
	}
}

// Default returns a default logger (info level, JSON format, stderr)
func Default() *Logger {
	return New(InfoLevel, "json", os.Stderr)
}

// IsDebug returns true if debug logging is enabled
func (l *Logger) IsDebug() bool {
	return l.level <= DebugLevel
}

// Debug logs a debug message with optional fields
func (l *Logger) Debug(msg string, fields ...interface{}) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, fields...)
	}
}

// Info logs an info message with optional fields
func (l *Logger) Info(msg string, fields ...interface{}) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, fields...)
	}
}

// Warn logs a warning message with optional fields
func (l *Logger) Warn(msg string, fields ...interface{}) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, fields...)
	}
}

// Error logs an error message with optional fields
func (l *Logger) Error(msg string, fields ...interface{}) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, fields...)
	}
}

func (l *Logger) log(level Level, msg string, fields ...interface{}) {
	entry := make(map[string]interface{})
	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	entry["level"] = level.String()
	entry["message"] = msg

	// Parse fields as key-value pairs
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprint(fields[i])
			entry[key] = fields[i+1]
		}
	}

	if l.format == "json" {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.writer, string(data))
	} else {
		// Simple text format
		fmt.Fprintf(l.writer, "[%s] %s: %s", entry["timestamp"], level.String(), msg)
		for k, v := range entry {
			if k != "timestamp" && k != "level" && k != "message" {
				fmt.Fprintf(l.writer, " %s=%v", k, v)
			}
		}
		fmt.Fprintln(l.writer)
	}
}
