package application_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/rss"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRefreshAllEmitsOneAggregateEventAndPreservesReadArticles(t *testing.T) {
	fetcher := rss.NewFetcher()
	fetcher.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Feed %s</title><link>https://example.com/</link>
<item><title>Already read</title><link>https://example.com/%s/old</link><guid>old-%s</guid></item>
<item><title>Actually new</title><link>https://example.com/%s/new</link><guid>new-%s</guid></item>
</channel></rss>`, req.URL.Path, req.URL.Path, req.URL.Path, req.URL.Path, req.URL.Path)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	feeds := sqlite.NewFeedRepo(db)
	articles := sqlite.NewArticleRepo(db)
	now := time.Now().UTC()
	feedList := make([]*domain.Feed, 0, 2)
	for _, path := range []string{"a", "b"} {
		feed := &domain.Feed{
			ID: uuid.NewString(), URL: "https://feeds.test/" + path, Title: path,
			PollIntervalSeconds: 3600, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		if err := feeds.Create(ctx, feed); err != nil {
			t.Fatal(err)
		}
		feedList = append(feedList, feed)
		if _, err := articles.UpsertMany(ctx, []domain.Article{{
			ID: uuid.NewString(), FeedID: feed.ID, Title: "Already read",
			URL: "https://example.com/" + path + "/old", ExternalID: "old-/" + path,
			DiscoveredAt: now, IsRead: true,
		}}); err != nil {
			t.Fatal(err)
		}
	}

	var addedEvents []map[string]any
	svc := &application.Service{
		Feeds: feeds, Articles: articles, Settings: sqlite.NewSettingsRepo(db), RSS: fetcher,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Emit: func(name string, payload any) {
			if name == "articles.added" {
				addedEvents = append(addedEvents, payload.(map[string]any))
			}
		},
	}
	if err := svc.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}

	if len(addedEvents) != 1 {
		t.Fatalf("expected one aggregate articles.added event across repeated refreshes, got %d", len(addedEvents))
	}
	if got := addedEvents[0]["count"]; got != 2 {
		t.Fatalf("expected aggregate count 2, got %v", got)
	}
	if _, exists := addedEvents[0]["feedId"]; exists {
		t.Fatal("aggregate event should not identify a single feed")
	}

	for i, feed := range feedList {
		old, err := articles.FindByExternalKey(ctx, feed.ID, "old-/"+[]string{"a", "b"}[i], "", "")
		if err != nil {
			t.Fatal(err)
		}
		if !old.IsRead {
			t.Fatalf("previously read article for feed %s became unread", feed.Title)
		}
	}
}
