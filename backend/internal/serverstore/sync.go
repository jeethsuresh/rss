package serverstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const maxSyncBatch = 1000

func (s *Store) SyncFeeds(ctx context.Context, userID string, req SyncRequest) (*SyncResponse, error) {
	if req.Cursor < 0 || len(req.Ops) > maxSyncBatch {
		return nil, domain.ErrInvalidParams
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, op := range req.Ops {
		if err := s.applyFeedOp(ctx, tx, userID, op); err != nil {
			return nil, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT sequence, op_id, feed_url, present, logical_clock, device_id,
		       COALESCE(client_created_at, '')
		FROM feed_sync_ops
		WHERE user_id = ? AND sequence > ?
		ORDER BY sequence ASC
		LIMIT ?`, userID, req.Cursor, maxSyncBatch+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &SyncResponse{Cursor: req.Cursor, Ops: []FeedOp{}}
	for rows.Next() {
		var op FeedOp
		var present int
		if err := rows.Scan(&op.Sequence, &op.OpID, &op.FeedURL, &present, &op.LogicalClock, &op.DeviceID, &op.ClientCreatedAt); err != nil {
			return nil, err
		}
		op.Present = present == 1
		if len(out.Ops) == maxSyncBatch {
			out.HasMore = true
			break
		}
		out.Ops = append(out.Ops, op)
		out.Cursor = op.Sequence
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) applyFeedOp(ctx context.Context, tx *sql.Tx, userID string, op FeedOp) error {
	if strings.TrimSpace(op.OpID) == "" || len(op.OpID) > 200 ||
		strings.TrimSpace(op.DeviceID) == "" || len(op.DeviceID) > 200 || op.LogicalClock <= 0 {
		return domain.ErrInvalidParams
	}
	feedURL, err := NormalizeURL(op.FeedURL)
	if err != nil {
		return err
	}
	now := s.now()
	feedID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO feeds(
			id, url, title, description, site_url, icon_url,
			last_error, etag, last_modified, poll_interval_seconds, enabled,
			created_at, updated_at, is_read_later
		) VALUES (?, ?, ?, '', '', '', '', '', '', 3600, 1, ?, ?, 0)
		ON CONFLICT(url) DO NOTHING`, feedID, feedURL, feedURL, formatTime(now), formatTime(now))
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM feeds WHERE url=?`, feedURL).Scan(&feedID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO feed_fetch_state(feed_id, next_fetch_at, updated_at)
		VALUES (?, ?, ?)`, feedID, formatTime(now), formatTime(now))
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO feed_sync_ops(
			op_id, user_id, feed_id, feed_url, present, logical_clock, device_id,
			client_created_at, received_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.OpID, userID, feedID, feedURL, boolInt(op.Present), op.LogicalClock, op.DeviceID,
		nullIfBlank(op.ClientCreatedAt), formatTime(now))
	if err != nil {
		return err
	}
	inserted, err := res.RowsAffected()
	if err != nil || inserted == 0 {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_feeds(
			user_id, feed_id, present, logical_clock, device_id, op_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, feed_id) DO UPDATE SET
			present=excluded.present,
			logical_clock=excluded.logical_clock,
			device_id=excluded.device_id,
			op_id=excluded.op_id,
			updated_at=excluded.updated_at
		WHERE excluded.logical_clock > user_feeds.logical_clock
		   OR (excluded.logical_clock = user_feeds.logical_clock AND excluded.device_id > user_feeds.device_id)
		   OR (excluded.logical_clock = user_feeds.logical_clock AND excluded.device_id = user_feeds.device_id
		       AND excluded.op_id > user_feeds.op_id)`,
		userID, feedID, boolInt(op.Present), op.LogicalClock, op.DeviceID, op.OpID,
		formatTime(now), formatTime(now))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		DELETE FROM user_feed_folders
		WHERE user_id=? AND feed_id=?
		  AND EXISTS (
		    SELECT 1 FROM user_feeds
		    WHERE user_id=? AND feed_id=? AND present=0
		  )`, userID, feedID, userID, feedID)
	return err
}

func nullIfBlank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (s *Store) ActiveFeedIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT feed_id FROM user_feeds WHERE user_id=? AND present=1 ORDER BY feed_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ListFeeds(ctx context.Context, userID string) ([]domain.Feed, error) {
	ids, err := s.ActiveFeedIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Feed, 0, len(ids))
	for _, id := range ids {
		feed, err := s.feeds.Get(ctx, id)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var enabled int
		err = s.db.SQL.QueryRowContext(ctx, `
			SELECT enabled, poll_interval_seconds FROM user_feeds
			WHERE user_id=? AND feed_id=? AND present=1`, userID, id).
			Scan(&enabled, &feed.PollIntervalSeconds)
		if err != nil {
			return nil, err
		}
		feed.Enabled = enabled == 1
		err = s.db.SQL.QueryRowContext(ctx, `
			SELECT COUNT(1)
			FROM articles a
			LEFT JOIN user_article_state uas ON uas.user_id=? AND uas.article_id=a.id
			WHERE a.feed_id=? AND COALESCE(uas.is_read, 0)=0`, userID, id).Scan(&feed.UnreadCount)
		if err != nil {
			return nil, fmt.Errorf("unread count: %w", err)
		}
		out = append(out, *feed)
	}
	return out, nil
}
