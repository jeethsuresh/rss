package syncclient

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/httpapi"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestClientPushesAndPullsFeedOps(t *testing.T) {
	ctx := context.Background()
	serverDB, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer serverDB.Close()
	store := serverstore.New(serverDB)
	handler := httpapi.New(store, &application.Service{}, slog.Default(), httpapi.Config{RegistrationEnabled: true})

	localDB, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = localDB.SQL.Exec(`
		INSERT INTO feeds(
			id, url, title, description, site_url, icon_url, last_error, etag,
			last_modified, poll_interval_seconds, enabled, created_at, updated_at, is_read_later
		) VALUES (?, 'https://example.com/local.xml', 'Local', '', '', '', '', '', '', 3600, 1, ?, ?, 0)`,
		uuid.NewString(), now, now)
	if err != nil {
		t.Fatal(err)
	}

	client := New(localDB, Config{
		ServerURL: "http://rss.test", Username: "sync-user", Password: "a secure password",
		AutoRegister: true,
	}, slog.Default())
	client.HTTP = &http.Client{Transport: handlerTransport{handler: handler}}
	first, err := client.SyncOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Pushed != 1 {
		t.Fatalf("want one pushed op, got %d", first.Pushed)
	}
	var userID string
	if err := serverDB.SQL.QueryRow(`SELECT id FROM users WHERE username='sync-user'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	_, err = store.SyncFeeds(ctx, userID, serverstore.SyncRequest{Cursor: first.Cursor, Ops: []serverstore.FeedOp{{
		OpID: uuid.NewString(), FeedURL: "https://example.com/remote.xml", Present: true,
		LogicalClock: time.Now().Add(time.Second).UnixMilli(), DeviceID: "remote-device",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.SyncOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.Pulled == 0 {
		t.Fatal("remote operation was not pulled")
	}
	var localFeeds int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM feeds WHERE is_read_later=0`).Scan(&localFeeds); err != nil {
		t.Fatal(err)
	}
	if localFeeds != 2 {
		t.Fatalf("want two local feeds after merge, got %d", localFeeds)
	}
}

type handlerTransport struct{ handler http.Handler }

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}
