package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type FeedRepo struct{ db *DB }

func NewFeedRepo(db *DB) *FeedRepo { return &FeedRepo{db: db} }

func (r *FeedRepo) List(ctx context.Context) ([]domain.Feed, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT f.id, f.url, f.title, f.description, f.site_url, f.icon_url,
		       f.last_success_at, f.last_attempt_at, f.last_error, f.etag, f.last_modified,
		       f.poll_interval_seconds, f.enabled, f.created_at, f.updated_at,
		       COALESCE((SELECT COUNT(1) FROM articles a WHERE a.feed_id = f.id AND a.is_read = 0), 0)
		FROM feeds f
		ORDER BY f.title COLLATE NOCASE ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FeedRepo) Get(ctx context.Context, id string) (*domain.Feed, error) {
	row := r.db.SQL.QueryRowContext(ctx, `
		SELECT f.id, f.url, f.title, f.description, f.site_url, f.icon_url,
		       f.last_success_at, f.last_attempt_at, f.last_error, f.etag, f.last_modified,
		       f.poll_interval_seconds, f.enabled, f.created_at, f.updated_at,
		       COALESCE((SELECT COUNT(1) FROM articles a WHERE a.feed_id = f.id AND a.is_read = 0), 0)
		FROM feeds f WHERE f.id = ?`, id)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FeedRepo) GetByURL(ctx context.Context, url string) (*domain.Feed, error) {
	row := r.db.SQL.QueryRowContext(ctx, `
		SELECT f.id, f.url, f.title, f.description, f.site_url, f.icon_url,
		       f.last_success_at, f.last_attempt_at, f.last_error, f.etag, f.last_modified,
		       f.poll_interval_seconds, f.enabled, f.created_at, f.updated_at,
		       COALESCE((SELECT COUNT(1) FROM articles a WHERE a.feed_id = f.id AND a.is_read = 0), 0)
		FROM feeds f WHERE f.url = ?`, url)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FeedRepo) Create(ctx context.Context, feed *domain.Feed) error {
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO feeds (
			id, url, title, description, site_url, icon_url,
			last_success_at, last_attempt_at, last_error, etag, last_modified,
			poll_interval_seconds, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		feed.ID, feed.URL, feed.Title, feed.Description, feed.SiteURL, feed.IconURL,
		nullTime(feed.LastSuccessAt), nullTime(feed.LastAttemptAt), feed.LastError, feed.ETag, feed.LastModified,
		feed.PollIntervalSeconds, boolToInt(feed.Enabled),
		feed.CreatedAt.UTC().Format(time.RFC3339Nano), feed.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (r *FeedRepo) Update(ctx context.Context, feed *domain.Feed) error {
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE feeds SET
			url=?, title=?, description=?, site_url=?, icon_url=?,
			last_success_at=?, last_attempt_at=?, last_error=?, etag=?, last_modified=?,
			poll_interval_seconds=?, enabled=?, updated_at=?
		WHERE id=?`,
		feed.URL, feed.Title, feed.Description, feed.SiteURL, feed.IconURL,
		nullTime(feed.LastSuccessAt), nullTime(feed.LastAttemptAt), feed.LastError, feed.ETag, feed.LastModified,
		feed.PollIntervalSeconds, boolToInt(feed.Enabled),
		feed.UpdatedAt.UTC().Format(time.RFC3339Nano), feed.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FeedRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.SQL.ExecContext(ctx, `DELETE FROM feeds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFeed(row rowScanner) (domain.Feed, error) {
	var f domain.Feed
	var lastSuccess, lastAttempt sql.NullString
	var enabled int
	var created, updated string
	err := row.Scan(
		&f.ID, &f.URL, &f.Title, &f.Description, &f.SiteURL, &f.IconURL,
		&lastSuccess, &lastAttempt, &f.LastError, &f.ETag, &f.LastModified,
		&f.PollIntervalSeconds, &enabled, &created, &updated, &f.UnreadCount,
	)
	if err != nil {
		return f, err
	}
	f.LastSuccessAt = parseTimePtr(lastSuccess)
	f.LastAttemptAt = parseTimePtr(lastAttempt)
	f.Enabled = enabled == 1
	f.CreatedAt = mustParseTime(created)
	f.UpdatedAt = mustParseTime(updated)
	return f, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
