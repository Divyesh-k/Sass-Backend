// Package logger wraps Go's standard log/slog so the whole service emits
// consistent structured (JSON in prod, text in dev) logs without pulling
// in a third-party logging dependency.
package logger

import (
	"log/slog"
	"os"
)

// New builds a slog.Logger. In production it emits JSON (so it's directly
// ingestible by CloudWatch/Loki/ELK); in development it emits a readable
// text format.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
