package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Service) AddReadLater(ctx context.Context, rawURL string) (*domain.Article, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, domain.ErrInvalidURL
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	feed, err := s.Feeds.EnsureReadLater(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	art := domain.Article{
		ID:           uuid.NewString(),
		FeedID:       feed.ID,
		Title:        rawURL,
		URL:          rawURL,
		DiscoveredAt: now,
		IsReadLater:  true,
		Priority:     domain.PriorityNone,
		CrawlStatus:  domain.CrawlPending,
	}
	if _, err := s.Articles.UpsertMany(ctx, []domain.Article{art}); err != nil {
		return nil, err
	}
	if s.Crawler != nil {
		_ = s.Crawler.CrawlOne(ctx, art.ID)
		if live, err := s.Crawler.FetchLive(ctx, art.ID); err == nil && live != "" {
			_ = s.Articles.SetLiveContent(ctx, art.ID, live)
		}
	}
	if s.AI != nil {
		s.AI.Enqueue(art.ID)
	}
	s.emit("articles.added", map[string]any{"feedId": feed.ID, "count": 1, "readLater": true})
	return s.Articles.Get(ctx, art.ID)
}

func (s *Service) ListReadLater(ctx context.Context) ([]domain.Article, error) {
	res, err := s.Articles.List(ctx, domain.ArticleQuery{ReadLaterOnly: true, Limit: 200})
	if err != nil {
		return nil, err
	}
	return res.Articles, nil
}

func (s *Service) RecrawlArticle(ctx context.Context, id string) (*domain.Article, error) {
	if s.Crawler == nil {
		return nil, domain.ErrInvalidParams
	}
	if err := s.Crawler.CrawlOne(ctx, id); err != nil {
		// still return article with failed status
		_ = err
	}
	return s.Articles.Get(ctx, id)
}

func (s *Service) FetchLiveArticle(ctx context.Context, id string) (*domain.Article, error) {
	if s.Crawler == nil {
		return nil, domain.ErrInvalidParams
	}
	if _, err := s.Crawler.FetchLive(ctx, id); err != nil {
		return nil, err
	}
	return s.Articles.Get(ctx, id)
}
