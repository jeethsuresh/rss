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

func (r *SportsCacheRepo) TryClaim(ctx context.Context, key, owner string, lease time.Duration) (bool, error) {
	now := time.Now().UTC()
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO shared_fetch_leases(cache_key, lease_owner, lease_until, updated_at)
		VALUES (?, '', ?, ?)`, key, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE shared_fetch_leases SET lease_owner=?, lease_until=?, updated_at=?
		WHERE cache_key=? AND (lease_until <= ? OR lease_owner=?)`,
		owner, now.Add(lease).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
		key, now.Format(time.RFC3339Nano), owner)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

func (r *SportsCacheRepo) Release(ctx context.Context, key, owner string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM shared_fetch_leases WHERE cache_key=? AND lease_owner=?`, key, owner)
	return err
}

var _ domain.SportsCacheRepository = (*SportsCacheRepo)(nil)
var _ domain.SharedFetchLeaseRepository = (*SportsCacheRepo)(nil)
