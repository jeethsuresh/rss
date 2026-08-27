package syncclient

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

type Config struct {
	ServerURL    string
	Username     string
	Password     string
	AutoRegister bool
	Interval     time.Duration
}

type Client struct {
	DB     *sqlite.DB
	Config Config
	HTTP   *http.Client
	Log    *slog.Logger
	Emit   func(name string, payload any)

	mu           sync.Mutex
	token        string
	tokenExpires time.Time
}

func New(db *sqlite.DB, config Config, log *slog.Logger) *Client {
	if config.Interval <= 0 {
		config.Interval = 5 * time.Minute
	}
	return &Client{
		DB: db, Config: config, Log: log,
		HTTP: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) Run(ctx context.Context) {
	_, _ = c.SyncAndReport(ctx)
	ticker := time.NewTicker(c.Config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = c.SyncAndReport(ctx)
		}
	}
}

func (c *Client) SyncAndReport(ctx context.Context) (*Result, error) {
	c.emit("started", "")
	result, err := c.SyncOnce(ctx)
	if err != nil {
		c.Log.Warn("feed sync", "err", err)
		c.recordStatus(ctx, err)
		c.emit("error", err.Error())
		return nil, err
	}
	c.recordStatus(ctx, nil)
	c.emit("finished", "", "pushed", result.Pushed, "pulled", result.Pulled, "cursor", result.Cursor,
		"statePushed", result.StatePushed, "statePulled", result.StatePulled, "stateCursor", result.StateCursor)
	return result, nil
}

type Result struct {
	Pushed      int
	Pulled      int
	Cursor      int64
	StatePushed int
	StatePulled int
	StateCursor int64
}

func (c *Client) SyncOnce(ctx context.Context) (*Result, error) {
	serverURL := strings.TrimRight(strings.TrimSpace(c.Config.ServerURL), "/")
	if serverURL == "" || strings.TrimSpace(c.Config.Username) == "" || c.Config.Password == "" {
		return nil, errors.New("sync requires server URL, username, and password")
	}
	token, err := c.login(ctx, serverURL)
	if err != nil {
		return nil, err
	}
	cursor, err := c.cursor(ctx, serverURL)
	if err != nil {
		return nil, err
	}
	result := &Result{Cursor: cursor}
	for {
		pending, err := c.pendingOps(ctx, 1000)
		if err != nil {
			return nil, err
		}
		response, err := c.exchange(ctx, serverURL, token, serverstore.SyncRequest{Cursor: result.Cursor, Ops: pending})
		var statusErr *remoteStatusError
		if errors.As(err, &statusErr) && statusErr.Status == http.StatusUnauthorized {
			c.clearToken()
			token, err = c.login(ctx, serverURL)
			if err == nil {
				response, err = c.exchange(ctx, serverURL, token, serverstore.SyncRequest{Cursor: result.Cursor, Ops: pending})
			}
		}
		if err != nil {
			return nil, err
		}
		if err := c.applyResponse(ctx, serverURL, pending, response); err != nil {
			return nil, err
		}
		result.Pushed += len(pending)
		result.Pulled += len(response.Ops)
		result.Cursor = response.Cursor
		if len(pending) == 0 && !response.HasMore {
			break
		}
	}
	stateResult, err := c.syncState(ctx, serverURL, token)
	if err != nil {
		return nil, err
	}
	result.StatePushed = stateResult.Pushed
	result.StatePulled = stateResult.Pulled
	result.StateCursor = stateResult.Cursor
	return result, nil
}

type authResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (c *Client) login(ctx context.Context, serverURL string) (string, error) {
	c.mu.Lock()
	if c.token != "" && c.tokenExpires.After(time.Now().Add(time.Minute)) {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()
	body := map[string]string{"username": c.Config.Username, "password": c.Config.Password}
	var auth authResponse
	status, err := c.doJSON(ctx, http.MethodPost, serverURL+"/v1/auth/login", "", body, &auth)
	if err == nil && status < 300 && auth.Token != "" {
		c.rememberToken(auth)
		return auth.Token, nil
	}
	if !c.Config.AutoRegister || status != http.StatusUnauthorized {
		if status == http.StatusUnauthorized {
			return "", errors.New("invalid username or password")
		}
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("login failed with status %d", status)
	}
	auth = authResponse{}
	status, err = c.doJSON(ctx, http.MethodPost, serverURL+"/v1/auth/register", "", body, &auth)
	if err != nil {
		if status == http.StatusConflict {
			return "", errors.New("that username already exists")
		}
		if status == http.StatusForbidden {
			return "", errors.New("account registration is disabled on this server")
		}
		return "", err
	}
	if status >= 300 || auth.Token == "" {
		return "", fmt.Errorf("registration failed with status %d", status)
	}
	c.rememberToken(auth)
	return auth.Token, nil
}

func (c *Client) rememberToken(auth authResponse) {
	expires := auth.ExpiresAt
	if expires.IsZero() {
		expires = time.Now().Add(24 * time.Hour)
	}
	c.mu.Lock()
	c.token = auth.Token
	c.tokenExpires = expires
	c.mu.Unlock()
}

func (c *Client) clearToken() {
	c.mu.Lock()
	c.token = ""
	c.tokenExpires = time.Time{}
	c.mu.Unlock()
}

func (c *Client) exchange(ctx context.Context, serverURL, token string, request serverstore.SyncRequest) (*serverstore.SyncResponse, error) {
	var response serverstore.SyncResponse
	status, err := c.doJSON(ctx, http.MethodPost, serverURL+"/v1/sync/feeds", token, request, &response)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("sync failed with status %d", status)
	}
	return &response, nil
}

func (c *Client) doJSON(ctx context.Context, method, url, token string, input, output any) (int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return res.StatusCode, &remoteStatusError{Status: res.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if output != nil {
		if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(output); err != nil {
			return res.StatusCode, err
		}
	}
	return res.StatusCode, nil
}

type remoteStatusError struct {
	Status int
	Body   string
}

func (e *remoteStatusError) Error() string {
	return fmt.Sprintf("remote status %d: %s", e.Status, e.Body)
}

func (c *Client) cursor(ctx context.Context, serverURL string) (int64, error) {
	_, err := c.DB.SQL.ExecContext(ctx, `
		INSERT OR IGNORE INTO local_sync_accounts(server_url, username, cursor)
		VALUES (?, ?, 0)`, serverURL, c.Config.Username)
	if err != nil {
		return 0, err
	}
	var cursor int64
	err = c.DB.SQL.QueryRowContext(ctx, `
		SELECT cursor FROM local_sync_accounts WHERE server_url=? AND username=?`, serverURL, c.Config.Username).Scan(&cursor)
	return cursor, err
}

func (c *Client) pendingOps(ctx context.Context, limit int) ([]serverstore.FeedOp, error) {
	rows, err := c.DB.SQL.QueryContext(ctx, `
		SELECT op_id, feed_url, present, logical_clock, device_id, created_at
		FROM local_feed_sync_ops WHERE pushed_at IS NULL
		ORDER BY sequence ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ops := []serverstore.FeedOp{}
	for rows.Next() {
		var op serverstore.FeedOp
		var present int
		if err := rows.Scan(&op.OpID, &op.FeedURL, &present, &op.LogicalClock, &op.DeviceID, &op.ClientCreatedAt); err != nil {
			return nil, err
		}
		op.Present = present == 1
		if normalized, err := serverstore.NormalizeURL(op.FeedURL); err == nil {
			op.FeedURL = normalized
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (c *Client) applyResponse(ctx context.Context, serverURL string, pushed []serverstore.FeedOp, response *serverstore.SyncResponse) error {
	tx, err := c.DB.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE local_sync_apply_guard SET applying=1 WHERE id=1`); err != nil {
		return err
	}
	for _, op := range response.Ops {
		if err := applyRemoteOp(ctx, tx, op); err != nil {
			return err
		}
	}
	for _, op := range pushed {
		if _, err := tx.ExecContext(ctx, `UPDATE local_feed_sync_ops SET pushed_at=? WHERE op_id=?`, nowText(), op.OpID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE local_sync_accounts SET cursor=?, last_sync_at=?, last_error=''
		WHERE server_url=? AND username=?`, response.Cursor, nowText(), serverURL, c.Config.Username); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE local_sync_apply_guard SET applying=0 WHERE id=1`); err != nil {
		return err
	}
	return tx.Commit()
}

func applyRemoteOp(ctx context.Context, tx *sql.Tx, op serverstore.FeedOp) error {
	feedURL, err := serverstore.NormalizeURL(op.FeedURL)
	if err != nil {
		return err
	}
	var currentClock int64
	var currentDevice, currentOp string
	err = tx.QueryRowContext(ctx, `
		SELECT logical_clock, device_id, op_id FROM local_feed_versions WHERE feed_url=?`, feedURL).
		Scan(&currentClock, &currentDevice, &currentOp)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && !wins(op.LogicalClock, op.DeviceID, op.OpID, currentClock, currentDevice, currentOp) {
		return recordRemoteOp(ctx, tx, feedURL, op)
	}
	feedID, storedURL, err := findLocalFeed(ctx, tx, feedURL)
	if err != nil {
		return err
	}
	if op.Present && feedID == "" {
		now := nowText()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO feeds(
				id, url, title, description, site_url, icon_url, last_error, etag,
				last_modified, poll_interval_seconds, enabled, created_at, updated_at, is_read_later
			) VALUES (?, ?, ?, '', '', '', '', '', '', 3600, 1, ?, ?, 0)`,
			uuid.NewString(), feedURL, feedURL, now, now)
		if err != nil {
			return err
		}
	} else if !op.Present && feedID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM feeds WHERE id=?`, feedID); err != nil {
			return err
		}
	}
	if storedURL != "" && storedURL != feedURL {
		_, _ = tx.ExecContext(ctx, `DELETE FROM local_feed_versions WHERE feed_url=?`, storedURL)
	}
	if err := recordRemoteOp(ctx, tx, feedURL, op); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(feed_url) DO UPDATE SET
			present=excluded.present, logical_clock=excluded.logical_clock,
			device_id=excluded.device_id, op_id=excluded.op_id`,
		feedURL, boolInt(op.Present), op.LogicalClock, op.DeviceID, op.OpID)
	return err
}

func findLocalFeed(ctx context.Context, tx *sql.Tx, canonical string) (id, storedURL string, err error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, url FROM feeds WHERE is_read_later=0`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	for rows.Next() {
		var candidateID, candidateURL string
		if err := rows.Scan(&candidateID, &candidateURL); err != nil {
			return "", "", err
		}
		normalized, err := serverstore.NormalizeURL(candidateURL)
		if err == nil && normalized == canonical {
			return candidateID, candidateURL, nil
		}
	}
	return "", "", rows.Err()
}

func recordRemoteOp(ctx context.Context, tx *sql.Tx, feedURL string, op serverstore.FeedOp) error {
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO local_feed_sync_ops(
			op_id, feed_url, present, logical_clock, device_id, created_at, pushed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`, op.OpID, feedURL, boolInt(op.Present), op.LogicalClock,
		op.DeviceID, firstNonBlank(op.ClientCreatedAt, nowText()), nowText())
	return err
}

func wins(clock int64, device, op string, currentClock int64, currentDevice, currentOp string) bool {
	return clock > currentClock ||
		(clock == currentClock && device > currentDevice) ||
		(clock == currentClock && device == currentDevice && op > currentOp)
}

func (c *Client) recordStatus(ctx context.Context, syncErr error) {
	serverURL := strings.TrimRight(strings.TrimSpace(c.Config.ServerURL), "/")
	message := ""
	if syncErr != nil {
		message = syncErr.Error()
	}
	_, _ = c.DB.SQL.ExecContext(ctx, `
		INSERT INTO local_sync_accounts(server_url, username, cursor, last_error)
		VALUES (?, ?, 0, ?)
		ON CONFLICT(server_url, username) DO UPDATE SET last_error=excluded.last_error`,
		serverURL, c.Config.Username, message)
}

func (c *Client) emit(phase, errText string, values ...any) {
	if c.Emit == nil {
		return
	}
	payload := map[string]any{"phase": phase}
	if errText != "" {
		payload["error"] = errText
	}
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if ok {
			payload[key] = values[i+1]
		}
	}
	c.Emit("sync.status", payload)
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
