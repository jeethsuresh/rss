package mlb_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/mlb"
)

func TestListTeamsAndSchedule(t *testing.T) {
	c := mlb.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	teams, err := c.ListTeams(ctx)
	if err != nil {
		t.Fatalf("teams: %v", err)
	}
	if len(teams) < 30 {
		t.Fatalf("expected many MLB teams, got %d", len(teams))
	}

	seasons, err := c.ListSeasons(ctx)
	if err != nil {
		t.Fatalf("seasons: %v", err)
	}
	if len(seasons) == 0 {
		t.Fatal("no seasons")
	}

	games, err := c.TeamSchedule(ctx, 147, seasons[0].SeasonID)
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if len(games) == 0 {
		t.Log("no games for current season yet; skipping detail")
		return
	}
	detail, err := c.GameDetail(ctx, games[0].ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Game.ID != games[0].ID {
		t.Fatalf("game id mismatch")
	}
}
