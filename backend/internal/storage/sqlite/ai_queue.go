package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type AIQueueRepo struct{ db *DB }

func NewAIQueueRepo(db *DB) *AIQueueRepo { return &AIQueueRepo{db: db} }

func (r *AIQueueRepo) Enqueue(ctx context.Context, articleID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO ai_queue (article_id, status, enqueued_at) VALUES (?, 'pending', ?)
		ON CONFLICT(article_id) DO UPDATE SET
			status = CASE WHEN ai_queue.status IN ('done', 'failed') THEN 'pending' ELSE ai_queue.status END,
			enqueued_at = CASE WHEN ai_queue.status IN ('done', 'failed') THEN excluded.enqueued_at ELSE ai_queue.enqueued_at END,
			last_error = CASE WHEN ai_queue.status IN ('done', 'failed') THEN '' ELSE ai_queue.last_error END,
			started_at = CASE WHEN ai_queue.status IN ('done', 'failed') THEN NULL ELSE ai_queue.started_at END,
			finished_at = CASE WHEN ai_queue.status IN ('done', 'failed') THEN NULL ELSE ai_queue.finished_at END`,
		articleID, now,
	)
	return err
}

func (r *AIQueueRepo) ClaimNext(ctx context.Context) (string, bool, error) {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback() }()

	var articleID string
	err = tx.QueryRowContext(ctx, `
		SELECT article_id FROM ai_queue
		WHERE status = 'pending'
		ORDER BY enqueued_at ASC
		LIMIT 1`).Scan(&articleID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
		UPDATE ai_queue SET status = 'running', started_at = ?
		WHERE article_id = ? AND status = 'pending'`, now, articleID)
	if err != nil {
		return "", false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", false, nil
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return articleID, true, nil
}

func (r *AIQueueRepo) MarkDone(ctx context.Context, articleID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE ai_queue SET status = 'done', finished_at = ?, last_error = '' WHERE article_id = ?`,
		now, articleID,
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

func (r *AIQueueRepo) MarkFailed(ctx context.Context, articleID string, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE ai_queue SET status = 'failed', finished_at = ?, last_error = ?, attempts = attempts + 1
		WHERE article_id = ?`,
		now, errMsg, articleID,
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

func (r *AIQueueRepo) ResetRunning(ctx context.Context) error {
	_, err := r.db.SQL.ExecContext(ctx, `
		UPDATE ai_queue SET status = 'pending', started_at = NULL WHERE status = 'running'`)
	return err
}

func (r *AIQueueRepo) Counts(ctx context.Context) (pending, running, done, failed int, err error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT status, COUNT(1) FROM ai_queue GROUP BY status`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return 0, 0, 0, 0, err
		}
		switch status {
		case "pending":
			pending = count
		case "running":
			running = count
		case "done":
			done = count
		case "failed":
			failed = count
		}
	}
	return pending, running, done, failed, rows.Err()
}

func (r *AIQueueRepo) ListRecent(ctx context.Context, limit int) ([]domain.AIQueueItem, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT article_id, status, attempts, last_error, enqueued_at
		FROM ai_queue
		ORDER BY enqueued_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AIQueueItem{}
	for rows.Next() {
		var item domain.AIQueueItem
		if err := rows.Scan(&item.ArticleID, &item.Status, &item.Attempts, &item.LastError, &item.EnqueuedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AIQueueRepo) RetryFailed(ctx context.Context) (int, error) {
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE ai_queue SET status = 'pending', last_error = '', started_at = NULL, finished_at = NULL
		WHERE status = 'failed'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

type AILogRepo struct{ db *DB }

func NewAILogRepo(db *DB) *AILogRepo { return &AILogRepo{db: db} }

func (r *AILogRepo) Append(ctx context.Context, entry domain.AILogEntry) error {
	id := entry.ID
	if id == "" {
		id = uuid.NewString()
	}
	ts := entry.TS
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO ai_logs (id, ts, level, article_id, message, detail)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, ts, entry.Level, nullString(entry.ArticleID), entry.Message, entry.Detail,
	)
	return err
}

func (r *AILogRepo) List(ctx context.Context, limit int) ([]domain.AILogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT id, ts, level, COALESCE(article_id, ''), message, detail
		FROM ai_logs
		ORDER BY ts DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AILogEntry{}
	for rows.Next() {
		var e domain.AILogEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.Level, &e.ArticleID, &e.Message, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
