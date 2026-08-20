package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestMigrationsAndFeedCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	feeds := sqlite.NewFeedRepo(db)
	ctx := context.Background()
	now := time.Now().UTC()
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/feed.xml",
		Title:               "Example",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}
	got, err := feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Example" {
		t.Fatalf("title %q", got.Title)
	}

	articles := sqlite.NewArticleRepo(db)
	a := domain.Article{
		ID:           uuid.NewString(),
		FeedID:       feed.ID,
		Title:        "Hello",
		URL:          "https://example.com/a",
		ExternalID:   "guid-1",
		DiscoveredAt: now,
	}
	n, err := articles.UpsertMany(ctx, []domain.Article{a})
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected insert, got %d", n)
	}
	// dedupe
	a2 := a
	a2.ID = uuid.NewString()
	a2.Title = "Hello updated"
	_, err = articles.UpsertMany(ctx, []domain.Article{a2})
	if err != nil {
		t.Fatal(err)
	}
	list, err := articles.List(ctx, domain.ArticleQuery{FeedID: feed.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Articles) != 1 {
		t.Fatalf("expected 1 article after dedupe, got %d", len(list.Articles))
	}
}
