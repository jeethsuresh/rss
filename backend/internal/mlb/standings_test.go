package mlb_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/mlb"
)

func TestStandingsSmoke(t *testing.T) {
	c := mlb.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, err := c.Standings(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sections) < 8 {
		t.Fatalf("expected 6 divisions + 2 WC, got %d", len(s.Sections))
	}
	if len(s.Sections[0].Teams) == 0 || s.Sections[0].Teams[0].Team.ID == 0 {
		t.Fatal("missing team rows")
	}
}
