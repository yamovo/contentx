package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/yamovo/contentx/internal/config"
)

// Setup initializes the global slog logger based on configuration.
func Setup(cfg config.LogConfig) {
	level := parseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// If file output is configured, add a file writer.
	if strings.ToLower(cfg.Output) == "file" && cfg.FilePath != "" {
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Warn("failed to open log file, using stdout", "path", cfg.FilePath, "error", err)
		} else {
			var w io.Writer
			if strings.ToLower(cfg.Format) == "json" {
				w = io.MultiWriter(os.Stdout, f)
			} else {
				w = io.MultiWriter(os.Stdout, f)
			}
			switch strings.ToLower(cfg.Format) {
			case "json":
				handler = slog.NewJSONHandler(w, opts)
			default:
				handler = slog.NewTextHandler(w, opts)
			}
		}
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// parseLevel converts a string level to slog.Level.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
