package mlb_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/mlb"
)

func TestGameDetailBoxscore(t *testing.T) {
	c := mlb.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	teams, err := c.ListTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatal(err)
	}
	games, err := c.TeamSchedule(ctx, teams[0].ID, 2025)
	if err != nil {
		t.Fatal(err)
	}
	var pk int
	for _, g := range games {
		if g.Status == domain.MlbFinal {
			pk = g.ID
			break
		}
	}
	if pk == 0 {
		t.Skip("no final game")
	}
	d, err := c.GameDetail(ctx, pk)
	if err != nil {
		t.Fatal(err)
	}
	if d.AwayBox == nil || d.HomeBox == nil {
		t.Fatalf("missing boxes")
	}
	if len(d.AwayBox.Batters)+len(d.HomeBox.Batters) == 0 {
		t.Fatal("no batters")
	}
	if len(d.AwayBox.Pitchers)+len(d.HomeBox.Pitchers) == 0 {
		t.Fatal("no pitchers")
	}
}
