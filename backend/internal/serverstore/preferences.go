package serverstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Store) ListFolders(ctx context.Context, userID string) ([]domain.Folder, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT id, name, created_at FROM user_folders
		WHERE user_id=? ORDER BY name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	out := []domain.Folder{}
	for rows.Next() {
		var folder domain.Folder
		var created string
		if err := rows.Scan(&folder.ID, &folder.Name, &created); err != nil {
			return nil, err
		}
		folder.CreatedAt = parseTime(created)
		folder.FeedIDs = []string{}
		out = append(out, folder)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		feedRows, err := s.db.SQL.QueryContext(ctx, `
			SELECT feed_id FROM user_feed_folders WHERE user_id=? AND folder_id=? ORDER BY feed_id`, userID, out[i].ID)
		if err != nil {
			return nil, err
		}
		for feedRows.Next() {
			var id string
			if err := feedRows.Scan(&id); err != nil {
				_ = feedRows.Close()
				return nil, err
			}
			out[i].FeedIDs = append(out[i].FeedIDs, id)
		}
		if err := feedRows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) CreateFolder(ctx context.Context, userID, name string) (*domain.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return nil, domain.ErrInvalidParams
	}
	folder := &domain.Folder{ID: uuid.NewString(), Name: name, CreatedAt: s.now(), FeedIDs: []string{}}
	_, err := s.db.SQL.ExecContext(ctx, `
		INSERT INTO user_folders(id, user_id, name, created_at) VALUES (?, ?, ?, ?)`,
		folder.ID, userID, folder.Name, formatTime(folder.CreatedAt))
	if err != nil {
		return nil, err
	}
	return folder, nil
}

func (s *Store) DeleteFolder(ctx context.Context, userID, folderID string) error {
	res, err := s.db.SQL.ExecContext(ctx, `DELETE FROM user_folders WHERE id=? AND user_id=?`, folderID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) AssignFolder(ctx context.Context, userID, folderID, feedID string, assigned bool) error {
	var exists int
	err := s.db.SQL.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM user_folders f
		JOIN user_feeds uf ON uf.user_id=f.user_id AND uf.feed_id=? AND uf.present=1
		WHERE f.id=? AND f.user_id=?`, feedID, folderID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return domain.ErrNotFound
	}
	if assigned {
		_, err = s.db.SQL.ExecContext(ctx, `
			INSERT OR IGNORE INTO user_feed_folders(user_id, folder_id, feed_id) VALUES (?, ?, ?)`,
			userID, folderID, feedID)
	} else {
		_, err = s.db.SQL.ExecContext(ctx, `
			DELETE FROM user_feed_folders WHERE user_id=? AND folder_id=? AND feed_id=?`,
			userID, folderID, feedID)
	}
	return err
}

func (s *Store) GetSettings(ctx context.Context, userID string) (*domain.Settings, error) {
	now := formatTime(s.now())
	_, err := s.db.SQL.ExecContext(ctx, `
		INSERT OR IGNORE INTO user_settings(user_id, updated_at) VALUES (?, ?)`, userID, now)
	if err != nil {
		return nil, err
	}
	settings := &domain.Settings{DefaultPollIntervalSeconds: 3600}
	var markRead, notifications int
	err = s.db.SQL.QueryRowContext(ctx, `
		SELECT theme, article_density, default_sort, mark_read_on_open,
		       notifications_enabled, read_later_chrome
		FROM user_settings WHERE user_id=?`, userID).
		Scan(&settings.Theme, &settings.ArticleDensity, &settings.DefaultSort, &markRead,
			&notifications, &settings.ReadLaterChrome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	settings.MarkReadOnOpen = markRead == 1
	settings.NotificationsEnabled = notifications == 1
	return settings, nil
}

func (s *Store) UpdateSettings(ctx context.Context, userID string, patch map[string]any) (*domain.Settings, error) {
	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if value, ok := patch["theme"].(string); ok && (value == "system" || value == "light" || value == "dark") {
		settings.Theme = value
	}
	if value, ok := patch["articleDensity"].(string); ok && (value == "comfortable" || value == "compact") {
		settings.ArticleDensity = value
	}
	if value, ok := patch["defaultSort"].(string); ok && (value == "newest" || value == "oldest") {
		settings.DefaultSort = value
	}
	if value, ok := patch["markReadOnOpen"].(bool); ok {
		settings.MarkReadOnOpen = value
	}
	if value, ok := patch["notificationsEnabled"].(bool); ok {
		settings.NotificationsEnabled = value
	}
	if value, ok := patch["readLaterChrome"].(string); ok && (value == "tabs" || value == "brandControl") {
		settings.ReadLaterChrome = value
	}
	_, err = s.db.SQL.ExecContext(ctx, `
		UPDATE user_settings SET theme=?, article_density=?, default_sort=?,
			mark_read_on_open=?, notifications_enabled=?, read_later_chrome=?, updated_at=?
		WHERE user_id=?`, settings.Theme, settings.ArticleDensity, settings.DefaultSort,
		boolInt(settings.MarkReadOnOpen), boolInt(settings.NotificationsEnabled), settings.ReadLaterChrome,
		formatTime(s.now()), userID)
	if err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, userID)
}
