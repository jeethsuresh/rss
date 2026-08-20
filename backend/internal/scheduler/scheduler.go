package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type Scheduler struct {
	svc      *application.Service
	feeds    domain.FeedRepository
	log      *slog.Logger
	mu       sync.Mutex
	inflight map[string]bool
	failures map[string]int
}

func New(svc *application.Service, feeds domain.FeedRepository, log *slog.Logger) *Scheduler {
	return &Scheduler{
		svc:      svc,
		feeds:    feeds,
		log:      log,
		inflight: map[string]bool{},
		failures: map[string]int{},
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	feeds, err := s.feeds.List(ctx)
	if err != nil {
		s.log.Error("scheduler list feeds", "err", err)
		return
	}
	now := time.Now().UTC()
	for _, f := range feeds {
		if !f.Enabled || f.IsReadLater {
			continue
		}
		interval := time.Duration(f.PollIntervalSeconds) * time.Second
		if interval < time.Minute {
			interval = time.Minute
		}
		if fails := s.failures[f.ID]; fails > 0 {
			backoff := time.Duration(min(fails, 6)) * time.Minute
			if interval < backoff {
				interval = backoff
			}
		}
		due := f.LastAttemptAt == nil || now.Sub(*f.LastAttemptAt) >= interval
		if !due {
			continue
		}
		s.refreshAsync(ctx, f.ID)
	}
}

func (s *Scheduler) refreshAsync(ctx context.Context, feedID string) {
	s.mu.Lock()
	if s.inflight[feedID] {
		s.mu.Unlock()
		return
	}
	s.inflight[feedID] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.inflight, feedID)
			s.mu.Unlock()
		}()
		_, err := s.svc.RefreshFeed(ctx, feedID)
		s.mu.Lock()
		if err != nil {
			s.failures[feedID]++
			s.log.Warn("scheduled refresh failed", "feedId", feedID, "err", err)
		} else {
			s.failures[feedID] = 0
		}
		s.mu.Unlock()
	}()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
