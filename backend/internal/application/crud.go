package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Service) ListFeeds(ctx context.Context) ([]domain.Feed, error) {
	return s.Feeds.List(ctx)
}

func (s *Service) GetFeed(ctx context.Context, id string) (*domain.Feed, error) {
	return s.Feeds.Get(ctx, id)
}

func (s *Service) RemoveFeed(ctx context.Context, id string) error {
	return s.Feeds.Delete(ctx, id)
}

func (s *Service) SetFeedEnabled(ctx context.Context, id string, enabled bool) (*domain.Feed, error) {
	feed, err := s.Feeds.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	feed.Enabled = enabled
	feed.UpdatedAt = time.Now().UTC()
	if err := s.Feeds.Update(ctx, feed); err != nil {
		return nil, err
	}
	return s.Feeds.Get(ctx, id)
}

func (s *Service) SetFeedPollInterval(ctx context.Context, id string, seconds int) (*domain.Feed, error) {
	if seconds < 60 {
		seconds = 60
	}
	feed, err := s.Feeds.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	feed.PollIntervalSeconds = seconds
	feed.UpdatedAt = time.Now().UTC()
	if err := s.Feeds.Update(ctx, feed); err != nil {
		return nil, err
	}
	return s.Feeds.Get(ctx, id)
}

func (s *Service) ListArticles(ctx context.Context, q domain.ArticleQuery) (domain.ArticleListResult, error) {
	if q.DefaultSort == "" {
		settings, err := s.Settings.Get(ctx)
		if err == nil {
			q.DefaultSort = settings.DefaultSort
		}
	}
	if !q.ReadLaterOnly {
		q.ExcludeReadLater = true
	}
	return s.Articles.List(ctx, q)
}

func (s *Service) GetArticle(ctx context.Context, id string) (*domain.Article, error) {
	return s.Articles.Get(ctx, id)
}

func (s *Service) MarkRead(ctx context.Context, id string, read bool) (*domain.Article, error) {
	a, err := s.Articles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	a.IsRead = read
	if err := s.Articles.Update(ctx, a); err != nil {
		return nil, err
	}
	s.emit("article.updated", map[string]any{"articleId": id, "isRead": read})
	return s.Articles.Get(ctx, id)
}

func (s *Service) ToggleStar(ctx context.Context, id string) (*domain.Article, error) {
	a, err := s.Articles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	a.IsStarred = !a.IsStarred
	if err := s.Articles.Update(ctx, a); err != nil {
		return nil, err
	}
	s.emit("article.updated", map[string]any{"articleId": id, "isStarred": a.IsStarred})
	return s.Articles.Get(ctx, id)
}

func (s *Service) ListFolders(ctx context.Context) ([]domain.Folder, error) {
	return s.Folders.List(ctx)
}

func (s *Service) CreateFolder(ctx context.Context, name string) (*domain.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.ErrInvalidParams
	}
	f := &domain.Folder{ID: uuid.NewString(), Name: name, CreatedAt: time.Now().UTC(), FeedIDs: []string{}}
	if err := s.Folders.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) RemoveFolder(ctx context.Context, id string) error {
	return s.Folders.Delete(ctx, id)
}

func (s *Service) AssignFeed(ctx context.Context, folderID, feedID string) error {
	if _, err := s.Folders.Get(ctx, folderID); err != nil {
		return err
	}
	if _, err := s.Feeds.Get(ctx, feedID); err != nil {
		return err
	}
	return s.Folders.AssignFeed(ctx, folderID, feedID)
}

func (s *Service) UnassignFeed(ctx context.Context, folderID, feedID string) error {
	return s.Folders.UnassignFeed(ctx, folderID, feedID)
}

func (s *Service) GetSettings(ctx context.Context) (*domain.Settings, error) {
	return s.Settings.Get(ctx)
}
