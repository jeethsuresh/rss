package openf1_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/openf1"
)

func TestF1StandingsSmoke(t *testing.T) {
	c := openf1.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s, err := c.Standings(ctx, 2024)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Drivers) < 10 {
		t.Fatalf("drivers %d", len(s.Drivers))
	}
	if len(s.Constructors) < 5 {
		t.Fatalf("constructors %d", len(s.Constructors))
	}
}
