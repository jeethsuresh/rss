package serverfetch

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

type FeedScheduler struct {
	Service *application.Service
	Store   *serverstore.Store
	Log     *slog.Logger
	Workers int
}

func (s *FeedScheduler) Run(ctx context.Context) {
	workers := s.Workers
	if workers <= 0 {
		workers = 4
	}
	for i := 0; i < workers; i++ {
		go s.worker(ctx, uuid.NewString())
	}
	<-ctx.Done()
}

func (s *FeedScheduler) worker(ctx context.Context, owner string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		s.runOne(ctx, owner)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *FeedScheduler) runOne(ctx context.Context, owner string) {
	claim, err := s.Store.ClaimDueFeed(ctx, owner, 3*time.Minute)
	if err != nil {
		s.Log.Error("claim feed fetch", "err", err)
		return
	}
	if claim == nil {
		return
	}
	started := time.Now()
	_, inserted, fetchErr := s.Service.RefreshFeedMeasured(ctx, claim.FeedID)
	_, completeErr := s.Store.CompleteFeedFetch(ctx, *claim, serverstore.FetchOutcome{
		NewItems: inserted,
		Duration: time.Since(started),
		Err:      fetchErr,
	})
	if fetchErr != nil {
		s.Log.Warn("server feed refresh", "feedId", claim.FeedID, "err", fetchErr)
	}
	if completeErr != nil {
		s.Log.Error("complete feed fetch", "feedId", claim.FeedID, "err", completeErr)
	}
	if fetchErr == nil {
		if err := s.Store.MaterializeAllState(ctx); err != nil {
			s.Log.Error("materialize synchronized article state", "feedId", claim.FeedID, "err", err)
		}
	}
}
