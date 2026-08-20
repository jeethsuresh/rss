package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type SportsCacheRepo struct{ db *DB }

func NewSportsCacheRepo(db *DB) *SportsCacheRepo { return &SportsCacheRepo{db: db} }

func (r *SportsCacheRepo) Get(ctx context.Context, key string) (payload []byte, fetchedAt time.Time, ok bool, err error) {
	var raw string
	var at string
	err = r.db.SQL.QueryRowContext(ctx, `
		SELECT payload, fetched_at FROM sports_cache WHERE cache_key = ?`, key).Scan(&raw, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, err
	}
	fetchedAt, err = time.Parse(time.RFC3339Nano, at)
	if err != nil {
		fetchedAt, err = time.Parse(time.RFC3339, at)
		if err != nil {
			fetchedAt = time.Now().UTC()
		}
	}
	return []byte(raw), fetchedAt, true, nil
}

func (r *SportsCacheRepo) Set(ctx context.Context, key string, payload []byte) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO sports_cache (cache_key, payload, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET payload = excluded.payload, fetched_at = excluded.fetched_at`,
		key, string(payload), now,
	)
	return err
}

func (r *SportsCacheRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM sports_cache WHERE cache_key = ?`, key)
	return err
}

var _ domain.SportsCacheRepository = (*SportsCacheRepo)(nil)
