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

	sessionKey := 0
	for _, race := range races {
		if race.Status == "completed" {
			sessionKey = race.SessionKey
			break
		}
	}
	if sessionKey == 0 {
		sessionKey = races[len(races)-1].SessionKey
	}
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
	if sessionKey < 1_000_000_000 && len(detail.Events) == 0 {
		t.Fatalf("expected race-control events")
	}

	current, err := c.ListRaces(ctx, time.Now().UTC().Year())
	if err != nil {
		t.Fatalf("ListRaces current: %v", err)
	}
	if len(current) == 0 {
		t.Fatal("expected current-season races during OpenF1 live lockout fallback")
	}
}
