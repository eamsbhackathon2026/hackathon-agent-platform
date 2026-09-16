// Package logger constructs the application's structured JSON logger.
package logger

import (
	"io"
	"log/slog"
)

// New writes structured logs at the requested level.
func New(output io.Writer, levelText string) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelText)); err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
}
