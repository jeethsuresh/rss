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
	return s.saveReadLater(ctx, rawURL, rawURL)
}

func (s *Service) AddReadLaterFromArticle(ctx context.Context, articleID string) (*domain.Article, error) {
	src, err := s.Articles.Get(ctx, articleID)
	if err != nil {
		return nil, err
	}
	url := strings.TrimSpace(src.URL)
	if url == "" {
		return nil, domain.ErrInvalidURL
	}
	title := strings.TrimSpace(src.Title)
	if title == "" {
		title = url
	}
	return s.saveReadLater(ctx, url, title)
}

func (s *Service) saveReadLater(ctx context.Context, pageURL, title string) (*domain.Article, error) {
	feed, err := s.Feeds.EnsureReadLater(ctx)
	if err != nil {
		return nil, err
	}
	fp := "url:" + pageURL
	if existing, err := s.Articles.FindByExternalKey(ctx, feed.ID, "", pageURL, fp); err == nil {
		// Unarchive if it was archived; refresh title if still a bare URL.
		if existing.ArchivedAt != nil {
			_ = s.Articles.SetArchived(ctx, existing.ID, false)
		}
		if existing.Title == existing.URL && title != "" && title != pageURL {
			existing.Title = title
			_ = s.Articles.Update(ctx, existing)
		}
		s.kickReadLaterFetch(existing.ID)
		return s.Articles.Get(ctx, existing.ID)
	}

	now := time.Now().UTC()
	art := domain.Article{
		ID:           uuid.NewString(),
		FeedID:       feed.ID,
		Title:        title,
		URL:          pageURL,
		DiscoveredAt: now,
		IsReadLater:  true,
		Priority:     domain.PriorityNone,
		CrawlStatus:  domain.CrawlPending,
	}
	if _, err := s.Articles.UpsertMany(ctx, []domain.Article{art}); err != nil {
		return nil, err
	}
	// Upsert conflict keeps the old primary key — resolve by fingerprint.
	saved, err := s.Articles.FindByExternalKey(ctx, feed.ID, "", pageURL, fp)
	if err != nil {
		saved, err = s.Articles.Get(ctx, art.ID)
		if err != nil {
			return nil, err
		}
	}
	s.kickReadLaterFetch(saved.ID)
	s.emit("articles.added", map[string]any{"feedId": feed.ID, "count": 1, "readLater": true})
	return saved, nil
}

func (s *Service) kickReadLaterFetch(articleID string) {
	if s.Crawler != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			_ = s.Crawler.CrawlOne(ctx, articleID)
			_, _ = s.Crawler.FetchLive(ctx, articleID)
			if s.Emit != nil {
				if a, err := s.Articles.Get(context.Background(), articleID); err == nil {
					s.Emit("article.updated", map[string]any{"articleId": articleID, "crawlStatus": a.CrawlStatus})
				}
			}
		}()
	}
	if s.AI != nil {
		s.AI.Enqueue(articleID)
	}
}

func (s *Service) ListReadLater(ctx context.Context, filter string, search string) ([]domain.Article, error) {
	q := domain.ArticleQuery{ReadLaterOnly: true, Limit: 200, Search: strings.TrimSpace(search)}
	switch filter {
	case "unread":
		q.UnreadOnly = true
		q.ExcludeArchived = true
	case "starred":
		q.StarredOnly = true
		q.ExcludeArchived = true
	case "archived":
		q.ArchivedOnly = true
	default:
		q.ExcludeArchived = true
	}
	res, err := s.Articles.List(ctx, q)
	if err != nil {
		return nil, err
	}
	return res.Articles, nil
}

func (s *Service) ArchiveReadLater(ctx context.Context, id string) (*domain.Article, error) {
	if err := s.Articles.SetArchived(ctx, id, true); err != nil {
		return nil, err
	}
	a, err := s.Articles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.emit("article.updated", map[string]any{"articleId": id, "archived": true})
	return a, nil
}

func (s *Service) UnarchiveReadLater(ctx context.Context, id string) (*domain.Article, error) {
	if err := s.Articles.SetArchived(ctx, id, false); err != nil {
		return nil, err
	}
	a, err := s.Articles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.emit("article.updated", map[string]any{"articleId": id, "archived": false})
	return a, nil
}

func (s *Service) RemoveReadLater(ctx context.Context, id string) error {
	a, err := s.Articles.Get(ctx, id)
	if err != nil {
		return err
	}
	if !a.IsReadLater {
		return domain.ErrInvalidParams
	}
	if err := s.Articles.Delete(ctx, id); err != nil {
		return err
	}
	s.emit("article.removed", map[string]any{"articleId": id})
	return nil
}

func (s *Service) RecrawlArticle(ctx context.Context, id string) (*domain.Article, error) {
	if s.Crawler == nil {
		return nil, domain.ErrInvalidParams
	}
	_ = s.Crawler.CrawlOne(ctx, id)
	return s.Articles.Get(ctx, id)
}

func (s *Service) SetArticleExtract(ctx context.Context, id, html string) (*domain.Article, error) {
	status := domain.ExtractFailed
	source := ""
	if strings.TrimSpace(html) != "" {
		status = domain.ExtractOK
		source = "js"
	}
	if err := s.Articles.SetExtract(ctx, id, html, status, source); err != nil {
		return nil, err
	}
	s.ClusterArticle(ctx, id)
	s.emit("article.updated", map[string]any{"articleId": id, "extractStatus": string(status)})
	return s.Articles.Get(ctx, id)
}

func (s *Service) PendingExtractIDs(ctx context.Context) ([]string, error) {
	arts, err := s.Articles.ListPendingExtract(ctx, 100)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(arts))
	for _, a := range arts {
		ids = append(ids, a.ID)
	}
	return ids, nil
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
