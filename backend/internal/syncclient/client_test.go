package syncclient

import (
	"context"
	"fmt"
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
	localFeedID := uuid.NewString()
	_, err = localDB.SQL.Exec(`
		INSERT INTO feeds(
			id, url, title, description, site_url, icon_url, last_error, etag,
			last_modified, poll_interval_seconds, enabled, created_at, updated_at, is_read_later
		) VALUES (?, 'https://example.com/local.xml', 'Local', '', '', '', '', '', '', 3600, 1, ?, ?, 0)`,
		localFeedID, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`
		INSERT INTO feeds(
			id, url, title, description, site_url, icon_url, last_error, etag,
			last_modified, poll_interval_seconds, enabled, created_at, updated_at, is_read_later
		) VALUES ('readlater-local', 'readlater://local', 'Read later', '', '', '', '', '', '', 0, 1, ?, ?, 1)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`
		INSERT INTO articles(id, feed_id, title, url, fingerprint, discovered_at, is_read_later)
		VALUES (?, 'readlater-local', 'Saved locally', 'https://example.com/saved',
		        'url:https://example.com/saved', ?, 1)`, uuid.NewString(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`INSERT INTO sports_followed_teams(team_id, created_at) VALUES (147, ?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`INSERT INTO folders(id, name, created_at) VALUES ('folder-local', 'Local folder', ?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`INSERT INTO feed_folders(folder_id, feed_id) VALUES ('folder-local', ?)`, localFeedID); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(`UPDATE settings SET theme='dark' WHERE id=1`); err != nil {
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
	if first.StatePushed < 5 {
		t.Fatalf("want local state pushed, got %d operations", first.StatePushed)
	}
	var userID string
	if err := serverDB.SQL.QueryRow(`SELECT id FROM users WHERE username='sync-user'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{
		`SELECT COUNT(1) FROM user_read_later WHERE user_id='` + userID + `'`:                1,
		`SELECT COUNT(1) FROM user_sports_followed_teams WHERE user_id='` + userID + `'`:     1,
		`SELECT COUNT(1) FROM user_folders WHERE user_id='` + userID + `'`:                   1,
		`SELECT COUNT(1) FROM user_feed_folders WHERE user_id='` + userID + `'`:              1,
		`SELECT COUNT(1) FROM user_settings WHERE user_id='` + userID + `' AND theme='dark'`: 1,
	} {
		var got int
		if err := serverDB.SQL.QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("server state query %q: got %d, want %d, err=%v", query, got, want, err)
		}
	}

	var serverFeedID string
	if err := serverDB.SQL.QueryRow(`SELECT id FROM feeds WHERE url='https://example.com/local.xml'`).Scan(&serverFeedID); err != nil {
		t.Fatal(err)
	}
	serverArticleID := uuid.NewString()
	if _, err := serverDB.SQL.Exec(`
		INSERT INTO articles(id, feed_id, title, url, fingerprint, discovered_at)
		VALUES (?, ?, 'Shared article', 'https://example.com/article', 'shared-fingerprint', ?)`,
		serverArticleID, serverFeedID, now); err != nil {
		t.Fatal(err)
	}
	localArticleID := uuid.NewString()
	if _, err := localDB.SQL.Exec(`
		INSERT INTO articles(id, feed_id, title, url, fingerprint, discovered_at)
		VALUES (?, ?, 'Shared article', 'https://example.com/article', 'shared-fingerprint', ?)`,
		localArticleID, localFeedID, now); err != nil {
		t.Fatal(err)
	}
	read, starred := true, true
	if _, err := store.SetArticleState(ctx, userID, serverArticleID, serverstore.ArticleStatePatch{
		IsRead: &read, IsStarred: &starred,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddReadLater(ctx, userID, "https://example.com/remote-saved"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetFollowedTeams(ctx, userID, "mlb", []string{"111"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateSettings(ctx, userID, map[string]any{"theme": "light"}); err != nil {
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
	if second.StatePulled == 0 {
		t.Fatal("remote state operations were not pulled")
	}
	var localFeeds int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM feeds WHERE is_read_later=0`).Scan(&localFeeds); err != nil {
		t.Fatal(err)
	}
	if localFeeds != 2 {
		t.Fatalf("want two local feeds after merge, got %d", localFeeds)
	}
	var localRead, localStarred int
	if err := localDB.SQL.QueryRow(`SELECT is_read, is_starred FROM articles WHERE id=?`, localArticleID).
		Scan(&localRead, &localStarred); err != nil || localRead != 1 || localStarred != 1 {
		t.Fatalf("article state did not materialize: read=%d starred=%d err=%v", localRead, localStarred, err)
	}
	var localReadLater int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM articles WHERE is_read_later=1`).Scan(&localReadLater); err != nil || localReadLater != 2 {
		t.Fatalf("read-later state did not materialize: count=%d err=%v", localReadLater, err)
	}
	var localTeam int
	if err := localDB.SQL.QueryRow(`SELECT team_id FROM sports_followed_teams`).Scan(&localTeam); err != nil || localTeam != 111 {
		t.Fatalf("sports state did not materialize: team=%d err=%v", localTeam, err)
	}
	var theme string
	if err := localDB.SQL.QueryRow(`SELECT theme FROM settings WHERE id=1`).Scan(&theme); err != nil || theme != "light" {
		t.Fatalf("settings did not materialize: theme=%q err=%v", theme, err)
	}
}

func TestClientDrainsStateQueueAcrossMultipleBatches(t *testing.T) {
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
	if _, err := localDB.SQL.Exec(`DELETE FROM local_state_sync_ops; DELETE FROM local_state_versions`); err != nil {
		t.Fatal(err)
	}
	var deviceID string
	if err := localDB.SQL.QueryRow(`SELECT device_id FROM local_sync_config WHERE id=1`).Scan(&deviceID); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO local_state_sync_ops(
		op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
	) VALUES (?, ?, ?, ?, 1, ?, ?, ?)`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 205; i++ {
		if _, err := localDB.SQL.Exec(insert, uuid.NewString(), "article_state",
			fmt.Sprintf("https://example.com/feed.xml\nfingerprint-%d", i),
			`{"isRead":true,"isStarred":false}`, 1000+i, deviceID, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := localDB.SQL.Exec(insert, uuid.NewString(), "read_later", "https://example.com/saved",
		`{"title":"Saved","isRead":false,"isStarred":false}`, 1300, deviceID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := localDB.SQL.Exec(insert, uuid.NewString(), "sports_team", "mlb\n147",
		`{"sport":"mlb","teamId":"147"}`, 1301, deviceID, now); err != nil {
		t.Fatal(err)
	}

	client := New(localDB, Config{
		ServerURL: "http://rss.test", Username: "batch-user", Password: "a secure password",
		AutoRegister: true,
	}, slog.Default())
	client.HTTP = &http.Client{Transport: handlerTransport{handler: handler}}
	result, err := client.SyncOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatePushed != 207 {
		t.Fatalf("pushed %d state operations, want 207", result.StatePushed)
	}
	var pending int
	if err := localDB.SQL.QueryRow(`SELECT COUNT(1) FROM local_state_sync_ops WHERE pushed_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("pending state operations=%d err=%v", pending, err)
	}
	var userID string
	if err := serverDB.SQL.QueryRow(`SELECT id FROM users WHERE username='batch-user'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{
		`SELECT COUNT(1) FROM state_sync_ops WHERE user_id='` + userID + `'`:             207,
		`SELECT COUNT(1) FROM user_read_later WHERE user_id='` + userID + `'`:            1,
		`SELECT COUNT(1) FROM user_sports_followed_teams WHERE user_id='` + userID + `'`: 1,
	} {
		var got int
		if err := serverDB.SQL.QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("server query %q: got %d, want %d, err=%v", query, got, want, err)
		}
	}
}

type handlerTransport struct{ handler http.Handler }

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}
