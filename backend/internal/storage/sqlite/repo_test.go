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

func TestReadLaterRemoveDeletesArticle(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	feeds := sqlite.NewFeedRepo(db)
	feed, err := feeds.EnsureReadLater(ctx)
	if err != nil {
		t.Fatal(err)
	}
	articles := sqlite.NewArticleRepo(db)
	now := time.Now().UTC()
	id := uuid.NewString()
	a := domain.Article{
		ID:           id,
		FeedID:       feed.ID,
		Title:        "https://example.com/rl",
		URL:          "https://example.com/rl",
		DiscoveredAt: now,
		IsReadLater:  true,
	}
	if _, err := articles.UpsertMany(ctx, []domain.Article{a}); err != nil {
		t.Fatal(err)
	}
	if err := articles.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := articles.Get(ctx, id); err != domain.ErrNotFound {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}
