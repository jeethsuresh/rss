package syncclient

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

type stateSyncResult struct {
	Pushed int
	Pulled int
	Cursor int64
}

func (c *Client) syncState(ctx context.Context, serverURL, token string) (*stateSyncResult, error) {
	cursor, err := c.stateCursor(ctx, serverURL)
	if err != nil {
		return nil, err
	}
	result := &stateSyncResult{Cursor: cursor}
	for {
		pending, err := c.pendingStateOps(ctx, 100)
		if err != nil {
			return nil, err
		}
		response, err := c.exchangeState(ctx, serverURL, token, serverstore.StateSyncRequest{
			Cursor: result.Cursor, Ops: pending,
		})
		var statusErr *remoteStatusError
		if errors.As(err, &statusErr) && statusErr.Status == http.StatusUnauthorized {
			c.clearToken()
			token, err = c.login(ctx, serverURL)
			if err == nil {
				response, err = c.exchangeState(ctx, serverURL, token, serverstore.StateSyncRequest{
					Cursor: result.Cursor, Ops: pending,
				})
			}
		}
		if err != nil {
			return nil, err
		}
		if err := c.applyStateResponse(ctx, serverURL, pending, response); err != nil {
			return nil, err
		}
		result.Pushed += len(pending)
		result.Pulled += len(response.Ops)
		result.Cursor = response.Cursor
		// A request is capped at 100 operations. Continue until both the local
		// append-only queue and the server response cursor are fully drained.
		if len(pending) == 0 && !response.HasMore {
			break
		}
	}
	return result, nil
}

func (c *Client) stateCursor(ctx context.Context, serverURL string) (int64, error) {
	_, err := c.DB.SQL.ExecContext(ctx, `
		INSERT OR IGNORE INTO local_sync_accounts(server_url, username, cursor, state_cursor)
		VALUES (?, ?, 0, 0)`, serverURL, c.Config.Username)
	if err != nil {
		return 0, err
	}
	var cursor int64
	err = c.DB.SQL.QueryRowContext(ctx, `
		SELECT state_cursor FROM local_sync_accounts WHERE server_url=? AND username=?`,
		serverURL, c.Config.Username).Scan(&cursor)
	return cursor, err
}

func (c *Client) pendingStateOps(ctx context.Context, limit int) ([]serverstore.StateOp, error) {
	rows, err := c.DB.SQL.QueryContext(ctx, `
		SELECT op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
		FROM local_state_sync_ops WHERE pushed_at IS NULL
		ORDER BY sequence ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ops := []serverstore.StateOp{}
	for rows.Next() {
		var op serverstore.StateOp
		var payload string
		var present int
		if err := rows.Scan(&op.OpID, &op.Kind, &op.ObjectKey, &payload, &present,
			&op.LogicalClock, &op.DeviceID, &op.ClientCreatedAt); err != nil {
			return nil, err
		}
		op.Payload = json.RawMessage(payload)
		op.Present = present == 1
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (c *Client) exchangeState(ctx context.Context, serverURL, token string, request serverstore.StateSyncRequest) (*serverstore.StateSyncResponse, error) {
	var response serverstore.StateSyncResponse
	status, err := c.doJSON(ctx, http.MethodPost, serverURL+"/v1/sync/state", token, request, &response)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("state sync failed with status %d", status)
	}
	return &response, nil
}

func (c *Client) applyStateResponse(ctx context.Context, serverURL string, pushed []serverstore.StateOp, response *serverstore.StateSyncResponse) error {
	tx, err := c.DB.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE local_sync_apply_guard SET applying=1 WHERE id=1`); err != nil {
		return err
	}
	for _, op := range response.Ops {
		if err := recordRemoteStateOp(ctx, tx, op); err != nil {
			return err
		}
	}
	for _, op := range pushed {
		if _, err := tx.ExecContext(ctx, `UPDATE local_state_sync_ops SET pushed_at=? WHERE op_id=?`, nowText(), op.OpID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE local_sync_accounts SET state_cursor=?, last_sync_at=?, last_error=''
		WHERE server_url=? AND username=?`, response.Cursor, nowText(), serverURL, c.Config.Username); err != nil {
		return err
	}
	if err := materializeLocalState(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE local_sync_apply_guard SET applying=0 WHERE id=1`); err != nil {
		return err
	}
	return tx.Commit()
}

func recordRemoteStateOp(ctx context.Context, tx *sql.Tx, op serverstore.StateOp) error {
	payload := string(op.Payload)
	if payload == "" {
		payload = "{}"
	}
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO local_state_sync_ops(
			op_id, kind, object_key, payload, present, logical_clock, device_id, created_at, pushed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, op.OpID, op.Kind, op.ObjectKey, payload,
		boolInt(op.Present), op.LogicalClock, op.DeviceID,
		firstNonBlank(op.ClientCreatedAt, nowText()), nowText())
	return err
}

type localStateRecord struct {
	Kind      string
	ObjectKey string
	Payload   string
	Present   bool
}

func materializeLocalState(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT kind, object_key, payload, present
		FROM local_state_versions ORDER BY kind, object_key`)
	if err != nil {
		return err
	}
	records := []localStateRecord{}
	for rows.Next() {
		var record localStateRecord
		var present int
		if err := rows.Scan(&record.Kind, &record.ObjectKey, &record.Payload, &present); err != nil {
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
		if err := materializeLocalRecord(ctx, tx, record); err != nil {
			return err
		}
	}
	return nil
}

func materializeLocalRecord(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	switch record.Kind {
	case "article_state":
		return materializeLocalArticleState(ctx, tx, record)
	case "read_later":
		return materializeLocalReadLater(ctx, tx, record)
	case "folder":
		return materializeLocalFolder(ctx, tx, record)
	case "folder_feed":
		return materializeLocalFolderFeed(ctx, tx, record)
	case "settings":
		return materializeLocalSettings(ctx, tx, record)
	case "sports_team":
		return materializeLocalSportsTeam(ctx, tx, record)
	default:
		return nil
	}
}

func materializeLocalArticleState(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	parts := strings.SplitN(record.ObjectKey, "\n", 2)
	if len(parts) != 2 {
		return nil
	}
	feedURL, err := serverstore.NormalizeURL(parts[0])
	if err != nil {
		return nil
	}
	feedID, _, err := findLocalFeed(ctx, tx, feedURL)
	if err != nil || feedID == "" {
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
		UPDATE articles SET is_read=?, is_starred=?
		WHERE feed_id=? AND fingerprint=? AND is_read_later=0`,
		boolInt(payload.IsRead), boolInt(payload.IsStarred), feedID, parts[1])
	return err
}

func materializeLocalReadLater(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	pageURL, err := serverstore.NormalizeURL(record.ObjectKey)
	if err != nil {
		return nil
	}
	if !record.Present {
		_, err := tx.ExecContext(ctx, `DELETE FROM articles WHERE is_read_later=1 AND url=?`, pageURL)
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
	now := nowText()
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO feeds(
			id, url, title, description, site_url, icon_url, last_error, etag, last_modified,
			poll_interval_seconds, enabled, created_at, updated_at, is_read_later
		) VALUES ('readlater-local', 'readlater://local', 'Read later', '', '', '', '', '', '', 0, 1, ?, ?, 1)`, now, now)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = pageURL
	}
	var archived any
	if payload.ArchivedAt != nil && strings.TrimSpace(*payload.ArchivedAt) != "" {
		archived = *payload.ArchivedAt
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO articles(
			id, feed_id, title, url, external_id, fingerprint, is_read, is_starred,
			discovered_at, priority, crawl_status, is_read_later, archived_at
		) VALUES (?, 'readlater-local', ?, ?, '', ?, ?, ?, ?, 'none', 'pending', 1, ?)
		ON CONFLICT(feed_id, fingerprint) DO UPDATE SET
			title=CASE WHEN articles.title=articles.url THEN excluded.title ELSE articles.title END,
			is_read=excluded.is_read, is_starred=excluded.is_starred,
			archived_at=excluded.archived_at`, uuid.NewString(), title, pageURL, "url:"+pageURL,
		boolInt(payload.IsRead), boolInt(payload.IsStarred), now, archived)
	return err
}

func materializeLocalFolder(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	if !record.Present {
		_, err := tx.ExecContext(ctx, `DELETE FROM folders WHERE id=?`, record.ObjectKey)
		return err
	}
	var payload struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil || strings.TrimSpace(payload.Name) == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO folders(id, name, created_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name`, record.ObjectKey, strings.TrimSpace(payload.Name), nowText())
	return err
}

func materializeLocalFolderFeed(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	var payload struct {
		FolderID string `json:"folderId"`
		FeedURL  string `json:"feedUrl"`
	}
	if json.Unmarshal([]byte(record.Payload), &payload) != nil {
		return nil
	}
	feedURL, err := serverstore.NormalizeURL(payload.FeedURL)
	if err != nil {
		return nil
	}
	feedID, _, err := findLocalFeed(ctx, tx, feedURL)
	if err != nil || feedID == "" {
		return err
	}
	if record.Present {
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO feed_folders(folder_id, feed_id)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM folders WHERE id=?)`, payload.FolderID, feedID, payload.FolderID)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM feed_folders WHERE folder_id=? AND feed_id=?`, payload.FolderID, feedID)
	}
	return err
}

func materializeLocalSettings(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
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
		UPDATE settings SET default_poll_interval_seconds=?, theme=?, article_density=?, default_sort=?, mark_read_on_open=?,
			notifications_enabled=?, read_later_chrome=? WHERE id=1`,
		defaultIntValue(payload.DefaultPollIntervalSeconds, 3600), defaultValue(payload.Theme, "system"), defaultValue(payload.ArticleDensity, "comfortable"),
		defaultValue(payload.DefaultSort, "newest"), boolInt(payload.MarkReadOnOpen),
		boolInt(payload.NotificationsEnabled), defaultValue(payload.ReadLaterChrome, "tabs"))
	return err
}

func materializeLocalSportsTeam(ctx context.Context, tx *sql.Tx, record localStateRecord) error {
	parts := strings.SplitN(record.ObjectKey, "\n", 2)
	if len(parts) != 2 || parts[0] != "mlb" {
		return nil
	}
	teamID, err := strconv.Atoi(parts[1])
	if err != nil || teamID <= 0 {
		return nil
	}
	if record.Present {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO sports_followed_teams(team_id, created_at) VALUES (?, ?)`, teamID, nowText())
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM sports_followed_teams WHERE team_id=?`, teamID)
	}
	return err
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultIntValue(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
