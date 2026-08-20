package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Service) ExportFeedURLs(ctx context.Context) (string, error) {
	feeds, err := s.Feeds.List(ctx)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(feeds))
	for _, f := range feeds {
		lines = append(lines, f.URL)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func (s *Service) ImportFeedURLs(ctx context.Context, text string) (*domain.FeedImportResult, error) {
	result := &domain.FeedImportResult{Errors: []string{}}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, err := s.AddFeed(ctx, line); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", line, err))
			continue
		}
		result.Added++
	}
	return result, nil
}

func (s *Service) ListStories(ctx context.Context) ([]domain.Story, error) {
	if s.Stories == nil {
		return []domain.Story{}, nil
	}
	return s.Stories.List(ctx)
}

func (s *Service) GetStory(ctx context.Context, id string) (*domain.Story, error) {
	return s.Stories.Get(ctx, id)
}

func (s *Service) MarkStoryRead(ctx context.Context, id string, read bool) (*domain.Story, error) {
	if err := s.Stories.CascadeFlags(ctx, id, &read, nil); err != nil {
		return nil, err
	}
	s.emit("story.updated", map[string]any{"storyId": id})
	return s.Stories.Get(ctx, id)
}

func (s *Service) ToggleStoryStar(ctx context.Context, id string) (*domain.Story, error) {
	st, err := s.Stories.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	starred := !st.IsStarred
	if err := s.Stories.CascadeFlags(ctx, id, nil, &starred); err != nil {
		return nil, err
	}
	s.emit("story.updated", map[string]any{"storyId": id})
	return s.Stories.Get(ctx, id)
}

func (s *Service) UpdateSettings(ctx context.Context, patch map[string]any) (*domain.Settings, error) {
	sset, err := s.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	if v, ok := patch["defaultPollIntervalSeconds"].(float64); ok {
		sset.DefaultPollIntervalSeconds = int(v)
	}
	if v, ok := patch["theme"].(string); ok {
		sset.Theme = v
	}
	if v, ok := patch["articleDensity"].(string); ok {
		sset.ArticleDensity = v
	}
	if v, ok := patch["defaultSort"].(string); ok {
		sset.DefaultSort = v
	}
	if v, ok := patch["markReadOnOpen"].(bool); ok {
		sset.MarkReadOnOpen = v
	}
	if v, ok := patch["notificationsEnabled"].(bool); ok {
		sset.NotificationsEnabled = v
	}
	if v, ok := patch["aiEnabled"].(bool); ok {
		sset.AIEnabled = v
	}
	if v, ok := patch["aiBaseUrl"].(string); ok {
		sset.AIBaseURL = strings.TrimSpace(v)
	}
	if v, ok := patch["aiModel"].(string); ok {
		sset.AIModel = strings.TrimSpace(v)
	}
	if err := s.Settings.Update(ctx, sset); err != nil {
		return nil, err
	}
	return s.Settings.Get(ctx)
}

// Keep compiler happy for time in this file if needed later
var _ = time.Time{}
