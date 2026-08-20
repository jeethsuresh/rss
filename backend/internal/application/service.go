package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/rss"
)

type EventEmitter func(name string, payload any)

type Service struct {
	Feeds    domain.FeedRepository
	Articles domain.ArticleRepository
	Folders  domain.FolderRepository
	Settings domain.SettingsRepository
	RSS      *rss.Fetcher
	Log      *slog.Logger
	Emit     EventEmitter
	Version  string
	DBPath   string
}

func (s *Service) emit(name string, payload any) {
	if s.Emit != nil {
		s.Emit(name, payload)
	}
}

func (s *Service) PreviewFeed(ctx context.Context, rawURL string) (*domain.FeedPreview, error) {
	feedURL, err := s.resolveFeedURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	res, err := s.RSS.Fetch(ctx, feedURL, "", "")
	if err != nil {
		return nil, err
	}
	preview := rss.PreviewFromFeed(feedURL, res.Feed)
	return &preview, nil
}

func (s *Service) AddFeed(ctx context.Context, rawURL string) (*domain.Feed, error) {
	feedURL, err := s.resolveFeedURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if existing, err := s.Feeds.GetByURL(ctx, feedURL); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	settings, err := s.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 feedURL,
		PollIntervalSeconds: settings.DefaultPollIntervalSeconds,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	res, err := s.RSS.Fetch(ctx, feedURL, "", "")
	feed.LastAttemptAt = &now
	if err != nil {
		feed.LastError = err.Error()
		feed.Title = feedURL
		if createErr := s.Feeds.Create(ctx, feed); createErr != nil {
			return nil, createErr
		}
		s.emit("feed.error", map[string]any{"feedId": feed.ID, "error": feed.LastError})
		return feed, nil
	}
	feed.Title = res.Feed.Title
	if feed.Title == "" {
		feed.Title = feedURL
	}
	feed.Description = res.Feed.Description
	feed.SiteURL = res.Feed.Link
	feed.ETag = res.ETag
	feed.LastModified = res.LastModified
	feed.LastSuccessAt = &now
	feed.LastError = ""
	if err := s.Feeds.Create(ctx, feed); err != nil {
		return nil, err
	}
	articles := rss.NormalizeArticles(feed.ID, res.Feed, now)
	for i := range articles {
		articles[i].ID = uuid.NewString()
		fp := rss.Fingerprint(articles[i].ExternalID, articles[i].URL, articles[i].Title, articles[i].PublishedAt)
		// store fingerprint via upsert helper in repo — set ExternalID path; repo computes from fields
		_ = fp
	}
	n, err := s.Articles.UpsertMany(ctx, withFingerprints(articles))
	if err != nil {
		return nil, err
	}
	s.emit("feed.updated", map[string]any{"feedId": feed.ID})
	if n > 0 {
		s.emit("articles.added", map[string]any{"feedId": feed.ID, "count": n})
	}
	return s.Feeds.Get(ctx, feed.ID)
}

func withFingerprints(articles []domain.Article) []domain.Article {
	// Fingerprint is derived in sqlite repo from ExternalID/URL/title; ensure ExternalID/URL set.
	return articles
}

func (s *Service) resolveFeedURL(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", domain.ErrInvalidURL
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	res, err := s.RSS.Fetch(ctx, rawURL, "", "")
	if err == nil && res.Feed != nil {
		return rawURL, nil
	}
	discovered, dErr := s.RSS.Discover(ctx, rawURL)
	if dErr != nil {
		if err != nil {
			return "", err
		}
		return "", dErr
	}
	return discovered, nil
}

func (s *Service) RefreshFeed(ctx context.Context, id string) (*domain.Feed, error) {
	feed, err := s.Feeds.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	feed.LastAttemptAt = &now
	feed.UpdatedAt = now
	res, err := s.RSS.Fetch(ctx, feed.URL, feed.ETag, feed.LastModified)
	if err != nil {
		feed.LastError = err.Error()
		_ = s.Feeds.Update(ctx, feed)
		s.emit("feed.error", map[string]any{"feedId": feed.ID, "error": feed.LastError})
		return feed, err
	}
	if res.NotModified {
		feed.LastSuccessAt = &now
		feed.LastError = ""
		_ = s.Feeds.Update(ctx, feed)
		s.emit("feed.updated", map[string]any{"feedId": feed.ID, "notModified": true})
		return feed, nil
	}
	if res.ETag != "" {
		feed.ETag = res.ETag
	}
	if res.LastModified != "" {
		feed.LastModified = res.LastModified
	}
	if res.Feed.Title != "" {
		feed.Title = res.Feed.Title
	}
	if res.Feed.Description != "" {
		feed.Description = res.Feed.Description
	}
	if res.Feed.Link != "" {
		feed.SiteURL = res.Feed.Link
	}
	feed.LastSuccessAt = &now
	feed.LastError = ""
	if err := s.Feeds.Update(ctx, feed); err != nil {
		return nil, err
	}
	articles := rss.NormalizeArticles(feed.ID, res.Feed, now)
	for i := range articles {
		articles[i].ID = uuid.NewString()
	}
	n, err := s.Articles.UpsertMany(ctx, articles)
	if err != nil {
		return nil, err
	}
	s.emit("feed.updated", map[string]any{"feedId": feed.ID})
	if n > 0 {
		s.emit("articles.added", map[string]any{"feedId": feed.ID, "count": n})
	}
	return s.Feeds.Get(ctx, feed.ID)
}

func (s *Service) RefreshAll(ctx context.Context) error {
	feeds, err := s.Feeds.List(ctx)
	if err != nil {
		return err
	}
	for _, f := range feeds {
		if !f.Enabled {
			continue
		}
		if _, err := s.RefreshFeed(ctx, f.ID); err != nil {
			s.Log.Warn("refresh failed", "feedId", f.ID, "err", err)
		}
	}
	return nil
}
