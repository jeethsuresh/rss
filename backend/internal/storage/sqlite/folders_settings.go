package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type FolderRepo struct{ db *DB }

func NewFolderRepo(db *DB) *FolderRepo { return &FolderRepo{db: db} }

func (r *FolderRepo) List(ctx context.Context) ([]domain.Folder, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT id, name, created_at FROM folders ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Folder
	for rows.Next() {
		var f domain.Folder
		var created string
		if err := rows.Scan(&f.ID, &f.Name, &created); err != nil {
			return nil, err
		}
		f.CreatedAt = mustParseTime(created)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FolderRepo) Get(ctx context.Context, id string) (*domain.Folder, error) {
	var f domain.Folder
	var created string
	err := r.db.SQL.QueryRowContext(ctx, `SELECT id, name, created_at FROM folders WHERE id = ?`, id).Scan(&f.ID, &f.Name, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt = mustParseTime(created)
	return &f, nil
}

func (r *FolderRepo) Create(ctx context.Context, folder *domain.Folder) error {
	_, err := r.db.SQL.ExecContext(ctx, `INSERT INTO folders(id, name, created_at) VALUES (?, ?, ?)`,
		folder.ID, folder.Name, folder.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (r *FolderRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.SQL.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FolderRepo) AssignFeed(ctx context.Context, folderID, feedID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `INSERT OR IGNORE INTO feed_folders(folder_id, feed_id) VALUES (?, ?)`, folderID, feedID)
	return err
}

func (r *FolderRepo) UnassignFeed(ctx context.Context, folderID, feedID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM feed_folders WHERE folder_id = ? AND feed_id = ?`, folderID, feedID)
	return err
}

func (r *FolderRepo) FeedIDsInFolder(ctx context.Context, folderID string) ([]string, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT feed_id FROM feed_folders WHERE folder_id = ?`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type SettingsRepo struct{ db *DB }

func NewSettingsRepo(db *DB) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) Get(ctx context.Context) (*domain.Settings, error) {
	var s domain.Settings
	var markRead, notif int
	err := r.db.SQL.QueryRowContext(ctx, `
		SELECT default_poll_interval_seconds, theme, article_density, default_sort, mark_read_on_open, notifications_enabled
		FROM settings WHERE id = 1`).Scan(
		&s.DefaultPollIntervalSeconds, &s.Theme, &s.ArticleDensity, &s.DefaultSort, &markRead, &notif,
	)
	if err != nil {
		return nil, err
	}
	s.MarkReadOnOpen = markRead == 1
	s.NotificationsEnabled = notif == 1
	return &s, nil
}

func (r *SettingsRepo) Update(ctx context.Context, settings *domain.Settings) error {
	_, err := r.db.SQL.ExecContext(ctx, `
		UPDATE settings SET
			default_poll_interval_seconds=?, theme=?, article_density=?, default_sort=?,
			mark_read_on_open=?, notifications_enabled=?
		WHERE id = 1`,
		settings.DefaultPollIntervalSeconds, settings.Theme, settings.ArticleDensity, settings.DefaultSort,
		boolToInt(settings.MarkReadOnOpen), boolToInt(settings.NotificationsEnabled),
	)
	return err
}
