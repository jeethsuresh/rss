package openf1_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/openf1"
)

func TestListRacesSmoke(t *testing.T) {
	c := openf1.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	races, err := c.ListRaces(ctx, 2024)
	if err != nil {
		t.Fatalf("ListRaces: %v", err)
	}
	if len(races) < 10 {
		t.Fatalf("expected many 2024 races, got %d", len(races))
	}
	if races[0].SessionKey == 0 || races[0].Name == "" {
		t.Fatalf("unexpected race: %+v", races[0])
	}

	// Prefer a known completed race (Bahrain 2024) to avoid flake if newest lacks results yet.
	sessionKey := 9472
	detail, err := c.RaceDetail(ctx, sessionKey)
	if err != nil {
		t.Fatalf("RaceDetail: %v", err)
	}
	if detail.Race.SessionKey != sessionKey {
		t.Fatalf("session mismatch")
	}
	if len(detail.Results) == 0 {
		t.Fatalf("expected results for completed race")
	}
	if len(detail.Events) == 0 {
		t.Fatalf("expected race-control events")
	}
}
