package sqlite

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type ErrorLogRepo struct{ db *DB }

func NewErrorLogRepo(db *DB) *ErrorLogRepo { return &ErrorLogRepo{db: db} }

func (r *ErrorLogRepo) Append(ctx context.Context, entry domain.ErrorLogEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.OccurredAt == "" {
		entry.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO error_logs(id, occurred_at, source, operation, message, detail)
		VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.OccurredAt, entry.Source, entry.Operation, entry.Message, entry.Detail,
	)
	return err
}

func (r *ErrorLogRepo) List(ctx context.Context, limit int) ([]domain.ErrorLogEntry, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT id, occurred_at, source, operation, message, detail
		FROM error_logs
		ORDER BY occurred_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ErrorLogEntry{}
	for rows.Next() {
		var entry domain.ErrorLogEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.OccurredAt,
			&entry.Source,
			&entry.Operation,
			&entry.Message,
			&entry.Detail,
		); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
