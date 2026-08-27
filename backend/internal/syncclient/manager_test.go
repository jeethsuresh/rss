package syncclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/httpapi"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestManagerConnectSyncAndDisconnect(t *testing.T) {
	serverDB, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer serverDB.Close()
	handler := httpapi.New(
		serverstore.New(serverDB), &application.Service{}, slog.Default(),
		httpapi.Config{RegistrationEnabled: true},
	)

	localDB, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := NewManager(localDB, slog.Default(), time.Hour)
	manager.HTTP = &http.Client{Transport: handlerTransport{handler: handler}}
	manager.Start(ctx)
	status, err := manager.Connect(ctx, ConnectRequest{
		ServerURL: "http://rss.test",
		Username:  "desktop-reader",
		Password:  "correct horse battery staple",
		Register:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.Username != "desktop-reader" || status.ServerURL != "http://rss.test" {
		t.Fatalf("unexpected connected status: %#v", status)
	}
	if status.LastSyncAt == "" {
		t.Fatalf("expected initial sync timestamp: %#v", status)
	}
	session, err := manager.Session()
	if err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.ServerURL != "http://rss.test" || session.Username != "desktop-reader" {
		t.Fatalf("unexpected session snapshot: %#v", session)
	}
	var users int
	if err := serverDB.SQL.QueryRow(`SELECT COUNT(1) FROM users WHERE username='desktop-reader'`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("registered users=%d err=%v", users, err)
	}

	status, err = manager.Disconnect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Connected {
		t.Fatalf("expected disconnected status: %#v", status)
	}

	restored := NewManager(localDB, slog.Default(), time.Hour)
	restored.HTTP = &http.Client{Transport: handlerTransport{handler: handler}}
	restored.Start(ctx)
	status, err = restored.Connect(ctx, ConnectRequest{
		ServerURL: session.ServerURL, Username: session.Username,
		SessionToken: session.Token, SessionExpiresAt: session.ExpiresAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected {
		t.Fatalf("saved session was not restored: %#v", status)
	}
	_, _ = restored.Disconnect(ctx)
}

func TestManagerRejectsInvalidServerURL(t *testing.T) {
	localDB, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()
	manager := NewManager(localDB, slog.Default(), time.Hour)
	if _, err := manager.Connect(context.Background(), ConnectRequest{
		ServerURL: "file:///tmp/server", Username: "reader", Password: "long enough password",
	}); err == nil {
		t.Fatal("expected invalid server URL error")
	}
}

func TestManagerAuthorityCommitsAndRollsBackMutations(t *testing.T) {
	serverDB, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer serverDB.Close()
	api := httpapi.New(
		serverstore.New(serverDB), &application.Service{}, slog.Default(),
		httpapi.Config{RegistrationEnabled: true},
	)
	gate := &rpcGateHandler{inner: api}

	localDB, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := NewManager(localDB, slog.Default(), time.Hour)
	manager.HTTP = &http.Client{Transport: handlerTransport{handler: gate}}
	manager.Start(ctx)
	if _, err := manager.Connect(ctx, ConnectRequest{
		ServerURL: "http://rss.test", Username: "authority-user",
		Password: "correct horse battery staple", Register: true,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := manager.RPC(ctx, "sports.followed.toggle", json.RawMessage(`{"teamId":147}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `[147]` {
		t.Fatalf("unexpected authority result: %s", result)
	}
	var localTeam int
	if err := localDB.SQL.QueryRow(`SELECT team_id FROM sports_followed_teams`).Scan(&localTeam); err != nil || localTeam != 147 {
		t.Fatalf("committed server mutation was not materialized locally: team=%d err=%v", localTeam, err)
	}
	var committed int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM local_authority_mutations WHERE status='committed'`).Scan(&committed); err != nil || committed != 1 {
		t.Fatalf("committed journal rows=%d err=%v", committed, err)
	}

	gate.setFailRPC(true)
	if _, err := manager.RPC(ctx, "readLater.add", json.RawMessage(`{"url":"https://example.com/rejected"}`)); err == nil {
		t.Fatal("expected rejected server mutation")
	}
	var rolledBack int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM local_authority_mutations WHERE status='rolled_back'`).Scan(&rolledBack); err != nil || rolledBack != 1 {
		t.Fatalf("rolled-back journal rows=%d err=%v", rolledBack, err)
	}
	var rejectedLocal int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM articles WHERE url='https://example.com/rejected'`).Scan(&rejectedLocal); err != nil || rejectedLocal != 0 {
		t.Fatalf("rejected mutation leaked into local state: count=%d err=%v", rejectedLocal, err)
	}
}

type rpcGateHandler struct {
	inner http.Handler
	mu    sync.RWMutex
	fail  bool
}

func (h *rpcGateHandler) setFailRPC(fail bool) {
	h.mu.Lock()
	h.fail = fail
	h.mu.Unlock()
}

func (h *rpcGateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	fail := h.fail
	h.mu.RUnlock()
	if fail && r.URL.Path == "/v1/rpc" {
		http.Error(w, `{"error":{"code":"UNAVAILABLE","message":"forced failure"}}`, http.StatusServiceUnavailable)
		return
	}
	h.inner.ServeHTTP(w, r)
}
