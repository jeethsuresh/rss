package sqlite

import (
	"context"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type SettingsRepo struct{ db *DB }

func NewSettingsRepo(db *DB) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) Get(ctx context.Context) (*domain.Settings, error) {
	var s domain.Settings
	var markRead, notif, aiEnabled int
	err := r.db.SQL.QueryRowContext(ctx, `
		SELECT default_poll_interval_seconds, theme, article_density, default_sort,
		       mark_read_on_open, notifications_enabled,
		       ai_enabled, ai_base_url, ai_model, read_later_chrome,
		       COALESCE(pandascore_api_token, ''), COALESCE(stratz_api_token, ''),
		       COALESCE(dota_provider, 'pandascore')
		FROM settings WHERE id = 1`).Scan(
		&s.DefaultPollIntervalSeconds, &s.Theme, &s.ArticleDensity, &s.DefaultSort,
		&markRead, &notif, &aiEnabled, &s.AIBaseURL, &s.AIModel, &s.ReadLaterChrome,
		&s.PandaScoreAPIToken, &s.StratzAPIToken, &s.DotaProvider,
	)
	if err != nil {
		return nil, err
	}
	s.MarkReadOnOpen = markRead == 1
	s.NotificationsEnabled = notif == 1
	s.AIEnabled = aiEnabled == 1
	if s.AIBaseURL == "" {
		s.AIBaseURL = "http://127.0.0.1:1234/v1"
	}
	if s.ReadLaterChrome == "" {
		s.ReadLaterChrome = "tabs"
	}
	if s.DotaProvider != "opendota" {
		s.DotaProvider = "pandascore"
	}
	return &s, nil
}

func (r *SettingsRepo) Update(ctx context.Context, settings *domain.Settings) error {
	chrome := settings.ReadLaterChrome
	if chrome != "brandControl" {
		chrome = "tabs"
	}
	provider := settings.DotaProvider
	if provider != "opendota" {
		provider = "pandascore"
	}
	_, err := r.db.SQL.ExecContext(ctx, `
		UPDATE settings SET
			default_poll_interval_seconds=?, theme=?, article_density=?, default_sort=?,
			mark_read_on_open=?, notifications_enabled=?,
			ai_enabled=?, ai_base_url=?, ai_model=?, read_later_chrome=?,
			pandascore_api_token=?, stratz_api_token=?, dota_provider=?
		WHERE id = 1`,
		settings.DefaultPollIntervalSeconds, settings.Theme, settings.ArticleDensity, settings.DefaultSort,
		boolToInt(settings.MarkReadOnOpen), boolToInt(settings.NotificationsEnabled),
		boolToInt(settings.AIEnabled), settings.AIBaseURL, settings.AIModel, chrome,
		settings.PandaScoreAPIToken, settings.StratzAPIToken, provider,
	)
	return err
}
