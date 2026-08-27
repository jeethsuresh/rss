package syncclient

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

var ErrNotConnected = errors.New("not connected to a sync server")

type ConnectRequest struct {
	ServerURL string `json:"serverUrl"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Register  bool   `json:"register"`
}

type Status struct {
	Connected  bool   `json:"connected"`
	ServerURL  string `json:"serverUrl"`
	Username   string `json:"username"`
	LastSyncAt string `json:"lastSyncAt,omitempty"`
	LastError  string `json:"lastError,omitempty"`
}

// Manager owns the desktop's runtime sync session. Credentials remain in
// process memory; only non-secret account and synchronization metadata is kept
// in the local database.
type Manager struct {
	DB       *sqlite.DB
	Log      *slog.Logger
	Interval time.Duration
	Emit     func(name string, payload any)
	HTTP     *http.Client

	mu        sync.Mutex
	syncMu    sync.Mutex
	rootCtx   context.Context
	client    *Client
	cancelRun context.CancelFunc
	serverURL string
	username  string
}

func NewManager(db *sqlite.DB, log *slog.Logger, interval time.Duration) *Manager {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Manager{DB: db, Log: log, Interval: interval, rootCtx: context.Background()}
}

func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	m.rootCtx = ctx
	m.mu.Unlock()
}

func (m *Manager) Connect(ctx context.Context, request ConnectRequest) (*Status, error) {
	serverURL, err := normalizeServerURL(request.ServerURL)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(request.Username)
	if username == "" || request.Password == "" {
		return nil, errors.New("server URL, username, and password are required")
	}

	client := New(m.DB, Config{
		ServerURL: serverURL, Username: username, Password: request.Password,
		AutoRegister: request.Register, Interval: m.Interval,
	}, m.Log)
	if m.HTTP != nil {
		client.HTTP = m.HTTP
	}
	client.Emit = m.Emit
	if _, err := m.syncClient(ctx, client); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.cancelRun != nil {
		m.cancelRun()
	}
	runCtx, cancel := context.WithCancel(m.rootCtx)
	m.client = client
	m.cancelRun = cancel
	m.serverURL = serverURL
	m.username = username
	m.mu.Unlock()

	go m.run(runCtx, client)
	return m.Status(ctx)
}

func (m *Manager) run(ctx context.Context, client *Client) {
	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.syncClient(ctx, client)
		}
	}
}

func (m *Manager) Disconnect(ctx context.Context) (*Status, error) {
	m.mu.Lock()
	if m.cancelRun != nil {
		m.cancelRun()
	}
	m.client = nil
	m.cancelRun = nil
	m.mu.Unlock()
	return m.Status(ctx)
}

func (m *Manager) SyncNow(ctx context.Context) (*Status, error) {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return nil, ErrNotConnected
	}
	if _, err := m.syncClient(ctx, client); err != nil {
		return nil, err
	}
	return m.Status(ctx)
}

func (m *Manager) syncClient(ctx context.Context, client *Client) (*Result, error) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	return client.SyncAndReport(ctx)
}

func (m *Manager) Status(ctx context.Context) (*Status, error) {
	m.mu.Lock()
	connected := m.client != nil
	serverURL := m.serverURL
	username := m.username
	m.mu.Unlock()

	status := &Status{Connected: connected, ServerURL: serverURL, Username: username}
	if serverURL == "" || username == "" {
		var lastSync, lastError sql.NullString
		err := m.DB.SQL.QueryRowContext(ctx, `
			SELECT server_url, username, last_sync_at, last_error
			FROM local_sync_accounts
			ORDER BY CASE WHEN last_sync_at IS NULL THEN 1 ELSE 0 END, last_sync_at DESC
			LIMIT 1`).Scan(&status.ServerURL, &status.Username, &lastSync, &lastError)
		if errors.Is(err, sql.ErrNoRows) {
			return status, nil
		}
		if err != nil {
			return nil, err
		}
		status.LastSyncAt = lastSync.String
		status.LastError = lastError.String
		return status, nil
	}

	var lastSync, lastError sql.NullString
	err := m.DB.SQL.QueryRowContext(ctx, `
		SELECT last_sync_at, last_error FROM local_sync_accounts
		WHERE server_url=? AND username=?`, serverURL, username).Scan(&lastSync, &lastError)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return nil, err
	}
	status.LastSyncAt = lastSync.String
	status.LastError = lastError.String
	return status, nil
}

func normalizeServerURL(raw string) (string, error) {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("enter a valid http or https server URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL cannot include a query or fragment")
	}
	return value, nil
}
