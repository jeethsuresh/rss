package serverstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const (
	maxStatePayload   = 16 << 10
	maxStateSyncBatch = 100
)

var stateKinds = map[string]bool{
	"article_state": true,
	"read_later":    true,
	"folder":        true,
	"folder_feed":   true,
	"settings":      true,
	"sports_team":   true,
}

func (s *Store) SyncState(ctx context.Context, userID string, req StateSyncRequest) (*StateSyncResponse, error) {
	if req.Cursor < 0 || len(req.Ops) > maxStateSyncBatch {
		return nil, domain.ErrInvalidParams
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, op := range req.Ops {
		if err := s.applyStateOp(ctx, tx, userID, op); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.MaterializeUserState(ctx, userID); err != nil {
		return nil, err
	}
	return s.pullStateOps(ctx, userID, req.Cursor)
}

func (s *Store) pullStateOps(ctx context.Context, userID string, cursor int64) (*StateSyncResponse, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT sequence, op_id, kind, object_key, payload, present, logical_clock,
		       device_id, COALESCE(client_created_at, '')
		FROM state_sync_ops
		WHERE user_id=? AND sequence>?
		ORDER BY sequence ASC LIMIT ?`, userID, cursor, maxStateSyncBatch+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &StateSyncResponse{Cursor: cursor, Ops: []StateOp{}}
	for rows.Next() {
		var op StateOp
		var payload string
		var present int
		if err := rows.Scan(&op.Sequence, &op.OpID, &op.Kind, &op.ObjectKey, &payload,
			&present, &op.LogicalClock, &op.DeviceID, &op.ClientCreatedAt); err != nil {
			return nil, err
		}
		if len(out.Ops) == maxStateSyncBatch {
			out.HasMore = true
			break
		}
		op.Payload = json.RawMessage(payload)
		op.Present = present == 1
		out.Ops = append(out.Ops, op)
		out.Cursor = op.Sequence
	}
	return out, rows.Err()
}

func (s *Store) applyStateOp(ctx context.Context, tx *sql.Tx, userID string, op StateOp) error {
	if !stateKinds[op.Kind] || strings.TrimSpace(op.OpID) == "" || len(op.OpID) > 200 ||
		strings.TrimSpace(op.ObjectKey) == "" || len(op.ObjectKey) > 4096 ||
		strings.TrimSpace(op.DeviceID) == "" || len(op.DeviceID) > 200 || op.LogicalClock <= 0 ||
		len(op.Payload) > maxStatePayload || !json.Valid(op.Payload) {
		return domain.ErrInvalidParams
	}
	payload := string(op.Payload)
	if payload == "" {
		payload = "{}"
	}
	now := formatTime(s.now())
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO state_sync_ops(
			op_id, user_id, kind, object_key, payload, present, logical_clock,
			device_id, client_created_at, received_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, op.OpID, userID, op.Kind, op.ObjectKey,
		payload, boolInt(op.Present), op.LogicalClock, op.DeviceID,
		nullIfBlank(op.ClientCreatedAt), now)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_sync_records(
			user_id, kind, object_key, payload, present, logical_clock, device_id, op_id, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, kind, object_key) DO UPDATE SET
			payload=excluded.payload, present=excluded.present,
			logical_clock=excluded.logical_clock, device_id=excluded.device_id,
			op_id=excluded.op_id, updated_at=excluded.updated_at
		WHERE excluded.logical_clock > user_sync_records.logical_clock
		   OR (excluded.logical_clock = user_sync_records.logical_clock
		       AND excluded.device_id > user_sync_records.device_id)
		   OR (excluded.logical_clock = user_sync_records.logical_clock
		       AND excluded.device_id = user_sync_records.device_id
		       AND excluded.op_id > user_sync_records.op_id)`,
		userID, op.Kind, op.ObjectKey, payload, boolInt(op.Present), op.LogicalClock,
		op.DeviceID, op.OpID, now)
	return err
}

func (s *Store) AppendState(ctx context.Context, userID, kind, objectKey string, payload any, present bool) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := s.now()
	logicalClock := now.UnixMilli()
	op := StateOp{
		OpID: uuid.NewString(), Kind: kind, ObjectKey: objectKey, Payload: raw,
		Present: present, LogicalClock: logicalClock, DeviceID: "server",
		ClientCreatedAt: formatTime(now),
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var currentClock int64
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(logical_clock, 0) FROM user_sync_records
		WHERE user_id=? AND kind=? AND object_key=?`, userID, kind, objectKey).Scan(&currentClock)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if currentClock >= op.LogicalClock {
		op.LogicalClock = currentClock + 1
	}
	if err := s.applyStateOp(ctx, tx, userID, op); err != nil {
		return err
	}
	return tx.Commit()
}

type storedStateRecord struct {
	UserID    string
	Kind      string
	ObjectKey string
	Payload   string
	Present   bool
}

func (s *Store) MaterializeUserState(ctx context.Context, userID string) error {
	return s.materializeRecords(ctx, `
		SELECT user_id, kind, object_key, payload, present
		FROM user_sync_records WHERE user_id=? ORDER BY kind, object_key`, userID)
}

func (s *Store) MaterializeAllState(ctx context.Context) error {
	return s.materializeRecords(ctx, `
		SELECT user_id, kind, object_key, payload, present
		FROM user_sync_records ORDER BY user_id, kind, object_key`)
}

func (s *Store) materializeRecords(ctx context.Context, query string, args ...any) error {
	rows, err := s.db.SQL.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	records := []storedStateRecord{}
	for rows.Next() {
		var record storedStateRecord
		var present int
		if err := rows.Scan(&record.UserID, &record.Kind, &record.ObjectKey, &record.Payload, &present); err != nil {
			_ = rows.Close()
			return err
		}
		record.Present = present == 1
		records = append(records, record)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, record := range records {
		tx, err := s.db.SQL.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := s.materializeStateRecord(ctx, tx, record); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) materializeStateRecord(ctx context.Context, tx *sql.Tx, record storedStateRecord) error {
	switch record.Kind {
	case "article_state":
		parts := strings.SplitN(record.ObjectKey, "\n", 2)
		if len(parts) != 2 {
			return nil
		}
		feedURL, err := NormalizeURL(parts[0])
		if err != nil {
			return nil
		}
		var articleID string
		err = tx.QueryRowContext(ctx, `
			SELECT a.id FROM articles a JOIN feeds f ON f.id=a.feed_id
			WHERE f.url=? AND a.fingerprint=?`, feedURL, parts[1]).Scan(&articleID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var payload struct {
			IsRead    bool `json:"isRead"`
			IsStarred bool `json:"isStarred"`
		}
		if json.Unmarshal([]byte(record.Payload), &payload) != nil {
			return nil
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_article_state(user_id, article_id, is_read, is_starred, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id, article_id) DO UPDATE SET
			  is_read=excluded.is_read, is_starred=excluded.is_starred, updated_at=excluded.updated_at`,
			record.UserID, articleID, boolInt(payload.IsRead), boolInt(payload.IsStarred), formatTime(s.now()))
		return err
	case "read_later":
		return s.materializeReadLaterRecord(ctx, tx, record)
	case "folder":
		return s.materializeFolderRecord(ctx, tx, record)
	case "folder_feed":
		return s.materializeFolderFeedRecord(ctx, tx, record)
	case "settings":
		return s.materializeSettingsRecord(ctx, tx, record)
	case "sports_team":
		parts := strings.SplitN(record.ObjectKey, "\n", 2)
		if len(parts) != 2 {
			return nil
		}
		if record.Present {
			_, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO user_sports_followed_teams(user_id, sport, team_id, created_at)
				VALUES (?, ?, ?, ?)`, record.UserID, parts[0], parts[1], formatTime(s.now()))
			return err
		}
		_, err := tx.ExecContext(ctx, `
			DELETE FROM user_sports_followed_teams WHERE user_id=? AND sport=? AND team_id=?`,
			record.UserID, parts[0], parts[1])
		return err
	}
	return nil
}

func (s *Store) materializeFolderRecord(ctx context.Context, tx *sql.Tx, record storedStateRecord) error {
	if !record.Present {
		_, err := tx.ExecContext(ctx, `DELETE FROM user_folders WHERE id=? AND user_id=?`, record.ObjectKey, record.UserID)
		return err
	}
	var payload struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil || strings.TrimSpace(payload.Name) == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_folders(id, user_id, name, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name
		WHERE user_folders.user_id=excluded.user_id`, record.ObjectKey, record.UserID, payload.Name, formatTime(s.now()))
	return err
}

func (s *Store) materializeFolderFeedRecord(ctx context.Context, tx *sql.Tx, record storedStateRecord) error {
	var payload struct {
		FolderID string `json:"folderId"`
		FeedURL  string `json:"feedUrl"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil {
		return nil
	}
	feedURL, err := NormalizeURL(payload.FeedURL)
	if err != nil {
		return nil
	}
	var feedID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM feeds WHERE url=?`, feedURL).Scan(&feedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if record.Present {
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO user_feed_folders(user_id, folder_id, feed_id)
			SELECT ?, ?, ? WHERE EXISTS (
			  SELECT 1 FROM user_folders WHERE id=? AND user_id=?
			)`, record.UserID, payload.FolderID, feedID, payload.FolderID, record.UserID)
	} else {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM user_feed_folders WHERE user_id=? AND folder_id=? AND feed_id=?`,
			record.UserID, payload.FolderID, feedID)
	}
	return err
}

func (s *Store) materializeSettingsRecord(ctx context.Context, tx *sql.Tx, record storedStateRecord) error {
	if !record.Present {
		return nil
	}
	var payload struct {
		DefaultPollIntervalSeconds int    `json:"defaultPollIntervalSeconds"`
		Theme                      string `json:"theme"`
		ArticleDensity             string `json:"articleDensity"`
		DefaultSort                string `json:"defaultSort"`
		MarkReadOnOpen             bool   `json:"markReadOnOpen"`
		NotificationsEnabled       bool   `json:"notificationsEnabled"`
		ReadLaterChrome            string `json:"readLaterChrome"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_settings(
		  user_id, default_poll_interval_seconds, theme, article_density, default_sort, mark_read_on_open,
		  notifications_enabled, read_later_chrome, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		  default_poll_interval_seconds=excluded.default_poll_interval_seconds,
		  theme=excluded.theme, article_density=excluded.article_density,
		  default_sort=excluded.default_sort, mark_read_on_open=excluded.mark_read_on_open,
		  notifications_enabled=excluded.notifications_enabled,
		  read_later_chrome=excluded.read_later_chrome, updated_at=excluded.updated_at`,
		record.UserID, defaultInt(payload.DefaultPollIntervalSeconds, 3600),
		defaultString(payload.Theme, "system"), defaultString(payload.ArticleDensity, "comfortable"),
		defaultString(payload.DefaultSort, "newest"), boolInt(payload.MarkReadOnOpen),
		boolInt(payload.NotificationsEnabled), defaultString(payload.ReadLaterChrome, "tabs"), formatTime(s.now()))
	return err
}

func (s *Store) materializeReadLaterRecord(ctx context.Context, tx *sql.Tx, record storedStateRecord) error {
	normalized, err := NormalizeURL(record.ObjectKey)
	if err != nil {
		return nil
	}
	var documentID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM web_documents WHERE normalized_url=?`, normalized).Scan(&documentID)
	if errors.Is(err, sql.ErrNoRows) && record.Present {
		documentID = uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO web_documents(id, normalized_url, url, crawl_status, next_fetch_at, created_at, updated_at)
			VALUES (?, ?, ?, 'pending', ?, ?, ?)`, documentID, normalized, normalized,
			formatTime(s.now()), formatTime(s.now()), formatTime(s.now()))
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !record.Present {
		if documentID == "" {
			return nil
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM user_read_later WHERE user_id=? AND document_id=?`, record.UserID, documentID)
		return err
	}
	var payload struct {
		Title      string  `json:"title"`
		IsRead     bool    `json:"isRead"`
		IsStarred  bool    `json:"isStarred"`
		ArchivedAt *string `json:"archivedAt"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil {
		return nil
	}
	if payload.Title != "" {
		_, _ = tx.ExecContext(ctx, `UPDATE web_documents SET title=CASE WHEN title='' THEN ? ELSE title END WHERE id=?`, payload.Title, documentID)
	}
	var archived any
	if payload.ArchivedAt != nil && strings.TrimSpace(*payload.ArchivedAt) != "" {
		archived = *payload.ArchivedAt
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_read_later(
		  id, user_id, document_id, is_read, is_starred, archived_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, document_id) DO UPDATE SET
		  is_read=excluded.is_read, is_starred=excluded.is_starred,
		  archived_at=excluded.archived_at, updated_at=excluded.updated_at`,
		uuid.NewString(), record.UserID, documentID, boolInt(payload.IsRead), boolInt(payload.IsStarred),
		archived, formatTime(s.now()), formatTime(s.now()))
	return err
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func articleStateKey(feedURL, fingerprint string) string { return feedURL + "\n" + fingerprint }
func sportsTeamKey(sport, id string) string              { return sport + "\n" + id }
