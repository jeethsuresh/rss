package application

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestSportsCachePersistsAcrossServiceRestarts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sports-cache.db")

	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := &SportsService{
		Cache:      sqlite.NewSportsCacheRepo(db),
		refreshing: map[string]bool{},
		cacheOwner: "first-server",
	}
	fetches := 0
	got, cached, err := getOrFetch(first, ctx, "mlb.standings.2026", time.Hour, "mlb.standings", nil,
		func(context.Context) (map[string]int, error) {
			fetches++
			return map[string]int{"wins": 92}, nil
		})
	if err != nil || cached || got["wins"] != 92 {
		t.Fatalf("initial fetch: value=%v cached=%v err=%v", got, cached, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	second := &SportsService{
		Cache:      sqlite.NewSportsCacheRepo(db),
		refreshing: map[string]bool{},
		cacheOwner: "second-server",
	}
	got, cached, err = getOrFetch(second, ctx, "mlb.standings.2026", time.Hour, "mlb.standings", nil,
		func(context.Context) (map[string]int, error) {
			fetches++
			return map[string]int{"wins": 0}, nil
		})
	if err != nil || !cached || got["wins"] != 92 {
		t.Fatalf("persisted fetch: value=%v cached=%v err=%v", got, cached, err)
	}
	if fetches != 1 {
		t.Fatalf("persisted cache called upstream again: fetches=%d", fetches)
	}
}
