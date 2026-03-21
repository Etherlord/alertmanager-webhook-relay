package logging

import (
	"io"
	"log/slog"
	"os"
)

// New creates a new slog.Logger with JSON output to stdout at the given level.
func New(level slog.Level) *slog.Logger {
	return NewWithWriter(level, os.Stdout)
}

// NewWithWriter creates a new slog.Logger with JSON output to the given writer.
// This is useful for testing where output needs to be captured.
func NewWithWriter(level slog.Level, w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}
