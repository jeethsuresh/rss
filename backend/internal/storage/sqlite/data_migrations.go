package sqlite

import (
	"context"
	"database/sql"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func runDataMigration(ctx context.Context, tx *sql.Tx, version int) error {
	switch version {
	case 13:
		_, err := sanitizeReadLaterURLs(ctx, tx)
		return err
	default:
		return nil
	}
}

func sanitizeReadLaterURLs(ctx context.Context, tx *sql.Tx) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, url FROM articles WHERE is_read_later = 1`)
	if err != nil {
		return 0, err
	}

	invalidIDs := make([]string, 0)
	for rows.Next() {
		var id, rawURL string
		if err := rows.Scan(&id, &rawURL); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if _, err := domain.NormalizeReadLaterURL(rawURL); err != nil {
			invalidIDs = append(invalidIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	for _, id := range invalidIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM articles WHERE id = ? AND is_read_later = 1`, id); err != nil {
			return 0, err
		}
	}
	return len(invalidIDs), nil
}
