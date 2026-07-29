package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yamovo/contentx/internal/config"
)

// withRestoredDefault snapshots the global slog default and restores it after
// the test, since Setup mutates process-global state.
func withRestoredDefault(t *testing.T) {
	t.Helper()
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"Error", slog.LevelError},
		{"", slog.LevelInfo},        // default
		{"unknown", slog.LevelInfo}, // fallback
	}
	for _, tc := range cases {
		if got := parseLevel(tc.in); got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSetup_LevelFiltering(t *testing.T) {
	withRestoredDefault(t)

	Setup(config.LogConfig{Level: "error", Format: "text"})

	ctx := context.Background()
	h := slog.Default().Handler()
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("info should be disabled when level=error")
	}
	if h.Enabled(ctx, slog.LevelWarn) {
		t.Error("warn should be disabled when level=error")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("error should be enabled when level=error")
	}
}

func TestSetup_DebugEnablesEverything(t *testing.T) {
	withRestoredDefault(t)

	Setup(config.LogConfig{Level: "debug", Format: "json"})

	if !slog.Default().Handler().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be enabled when level=debug")
	}
}

func TestSetup_FileOutput_WritesToFile(t *testing.T) {
	withRestoredDefault(t)

	// Setup keeps the log file handle open for the process lifetime, so
	// t.TempDir()'s strict cleanup would fail on Windows (file in use).
	// Manage the temp dir manually and tolerate cleanup failure.
	dir, err := os.MkdirTemp("", "contentx-logger-test")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "app.log")
	Setup(config.LogConfig{Level: "info", Format: "json", Output: "file", FilePath: path})

	slog.Info("file output smoke", "marker", "logger-test-42")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "file output smoke") || !strings.Contains(content, "logger-test-42") {
		t.Errorf("log file missing expected entry, got: %s", content)
	}
	// json format should produce a JSON object line.
	if !strings.Contains(content, `"msg"`) {
		t.Errorf("expected JSON-formatted log line, got: %s", content)
	}
}

func TestSetup_FileOutput_InvalidPathFallsBack(t *testing.T) {
	withRestoredDefault(t)

	// Directory used as file path — OpenFile fails; Setup must not panic
	// and must still install a usable stdout logger.
	Setup(config.LogConfig{Level: "info", Format: "text", Output: "file", FilePath: t.TempDir()})

	if !slog.Default().Handler().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("fallback logger should still be usable at info level")
	}
}
