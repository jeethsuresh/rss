package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func TestSanitizeReadLaterURLsRemovesOnlyInvalidReadLaterRows(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	feed, err := NewFeedRepo(db).EnsureReadLater(ctx)
	if err != nil {
		t.Fatal(err)
	}
	articles := NewArticleRepo(db)
	now := time.Now().UTC()
	rows := []domain.Article{
		{ID: "valid-read-later", FeedID: feed.ID, Title: "Valid", URL: "https://example.com/article", DiscoveredAt: now, IsReadLater: true},
		{ID: "invalid-read-later", FeedID: feed.ID, Title: "Invalid", URL: "https://not a url", DiscoveredAt: now, IsReadLater: true},
		{ID: "invalid-regular", FeedID: feed.ID, Title: "Regular", URL: "https://also not a url", DiscoveredAt: now},
	}
	if _, err := articles.UpsertMany(ctx, rows); err != nil {
		t.Fatal(err)
	}

	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := sanitizeReadLaterURLs(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	if _, err := articles.Get(ctx, "valid-read-later"); err != nil {
		t.Fatalf("valid Read Later row was removed: %v", err)
	}
	if _, err := articles.Get(ctx, "invalid-read-later"); err != domain.ErrNotFound {
		t.Fatalf("invalid Read Later row still exists: %v", err)
	}
	if _, err := articles.Get(ctx, "invalid-regular"); err != nil {
		t.Fatalf("non-Read Later row was removed: %v", err)
	}
}
