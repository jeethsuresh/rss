package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestConfigureAIRequiresServerEnv(t *testing.T) {
	t.Setenv("RSS_SERVER_AI_ENABLED", "")
	t.Setenv("RSS_SERVER_AI_BASE_URL", "")
	t.Setenv("RSS_SERVER_AI_MODEL", "")
	db := openAIConfigDB(t)
	settings := sqlite.NewSettingsRepo(db)
	ctx := context.Background()
	current, err := settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	current.AIEnabled = true
	current.AIBaseURL = "http://10.1.2.6:1234/v1"
	current.AIModel = "local"
	if err := settings.Update(ctx, current); err != nil {
		t.Fatal(err)
	}

	configureAI(settings, slog.New(slog.NewTextHandler(io.Discard, nil)))

	enabled, base, model := readAIConfig(t, db)
	if enabled != 0 || base != "" || model != "" {
		t.Fatalf("AI config = enabled:%d base:%q model:%q", enabled, base, model)
	}
}

func TestConfigureAIUsesServerEnv(t *testing.T) {
	t.Setenv("RSS_SERVER_AI_ENABLED", "true")
	t.Setenv("RSS_SERVER_AI_BASE_URL", "https://ai.example/v1")
	t.Setenv("RSS_SERVER_AI_MODEL", "reader")
	db := openAIConfigDB(t)
	settings := sqlite.NewSettingsRepo(db)
	configureAI(settings, slog.New(slog.NewTextHandler(io.Discard, nil)))

	enabled, base, model := readAIConfig(t, db)
	if enabled != 1 || base != "https://ai.example/v1" || model != "reader" {
		t.Fatalf("AI config = enabled:%d base:%q model:%q", enabled, base, model)
	}
}

func TestConfigureAIStaysOffWithoutBaseURL(t *testing.T) {
	t.Setenv("RSS_SERVER_AI_ENABLED", "true")
	t.Setenv("RSS_SERVER_AI_BASE_URL", "")
	t.Setenv("RSS_SERVER_AI_MODEL", "reader")
	db := openAIConfigDB(t)
	settings := sqlite.NewSettingsRepo(db)
	configureAI(settings, slog.New(slog.NewTextHandler(io.Discard, nil)))

	enabled, base, model := readAIConfig(t, db)
	if enabled != 0 || base != "" || model != "reader" {
		t.Fatalf("AI config = enabled:%d base:%q model:%q", enabled, base, model)
	}
}

func openAIConfigDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func readAIConfig(t *testing.T, db *sqlite.DB) (int, string, string) {
	t.Helper()
	var enabled int
	var base, model string
	if err := db.SQL.QueryRow(`SELECT ai_enabled, ai_base_url, ai_model FROM settings WHERE id=1`).Scan(&enabled, &base, &model); err != nil {
		t.Fatal(err)
	}
	return enabled, base, model
}
