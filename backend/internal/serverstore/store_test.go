package serverstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func openTestStore(t *testing.T) (*Store, *sqlite.DB) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), db
}

func insertTestUser(t *testing.T, db *sqlite.DB, username string) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.SQL.Exec(`
		INSERT INTO users(id, username, password_hash, created_at, updated_at)
		VALUES (?, ?, 'test-only', ?, ?)`, id, username, now, now); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPasswordHashIsSaltedAndVerifies(t *testing.T) {
	first, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("password hashes should use unique salts")
	}
	if !verifyPassword(first, "correct horse battery staple") {
		t.Fatal("valid password did not verify")
	}
	if verifyPassword(first, "wrong password") {
		t.Fatal("invalid password verified")
	}
}

func TestFeedSyncDeduplicatesGloballyAndUsesLWW(t *testing.T) {
	store, db := openTestStore(t)
	ctx := context.Background()
	firstUser := insertTestUser(t, db, "first")
	secondUser := insertTestUser(t, db, "second")

	add := func(user, opID, device string, clock int64, present bool) {
		t.Helper()
		_, err := store.SyncFeeds(ctx, user, SyncRequest{Ops: []FeedOp{{
			OpID: opID, DeviceID: device, LogicalClock: clock,
			FeedURL: "HTTPS://Example.com:443/feed.xml#fragment", Present: present,
		}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(firstUser, "op-1", "laptop", 100, true)
	add(secondUser, "op-2", "phone", 100, true)

	var feeds, fetchStates int
	if err := db.SQL.QueryRow(`SELECT COUNT(1) FROM feeds WHERE is_read_later=0`).Scan(&feeds); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL.QueryRow(`SELECT COUNT(1) FROM feed_fetch_state`).Scan(&fetchStates); err != nil {
		t.Fatal(err)
	}
	if feeds != 1 || fetchStates != 1 {
		t.Fatalf("want one canonical feed and fetch state, got feeds=%d states=%d", feeds, fetchStates)
	}

	add(firstUser, "stale-delete", "laptop", 99, false)
	list, err := store.ListFeeds(ctx, firstUser)
	if err != nil || len(list) != 1 {
		t.Fatalf("stale delete won: feeds=%d err=%v", len(list), err)
	}
	add(firstUser, "new-delete", "laptop", 101, false)
	list, err = store.ListFeeds(ctx, firstUser)
	if err != nil || len(list) != 0 {
		t.Fatalf("new delete lost: feeds=%d err=%v", len(list), err)
	}
	other, err := store.ListFeeds(ctx, secondUser)
	if err != nil || len(other) != 1 {
		t.Fatalf("tenant delete leaked: feeds=%d err=%v", len(other), err)
	}
}

func TestReadLaterSharesDocumentButNotUserState(t *testing.T) {
	store, db := openTestStore(t)
	ctx := context.Background()
	first := insertTestUser(t, db, "first")
	second := insertTestUser(t, db, "second")
	item1, err := store.AddReadLater(ctx, first, "https://example.com/story")
	if err != nil {
		t.Fatal(err)
	}
	item2, err := store.AddReadLater(ctx, second, "https://example.com/story#section")
	if err != nil {
		t.Fatal(err)
	}
	if item1.SharedDocumentID != item2.SharedDocumentID {
		t.Fatal("same normalized URL did not share a document")
	}
	read := true
	if _, err := store.SetReadLaterState(ctx, first, item1.ID, ReadLaterPatch{IsRead: &read}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.GetReadLater(ctx, second, item2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.IsRead {
		t.Fatal("read-later state leaked between users")
	}
}

func TestAdaptiveDelayRespondsToVolumeAndFailures(t *testing.T) {
	now := time.Now().UTC()
	last := now.Add(-time.Hour)
	fast, _, _, _, _ := adaptiveDelay(now, 3600, &last, 0, 0, 12, false)
	slow, _, _, _, _ := adaptiveDelay(now, 3600, &last, 4, 0, 0, false)
	failed, _, _, _, failures := adaptiveDelay(now, 3600, &last, 0, 2, 0, true)
	if fast >= slow {
		t.Fatalf("high-volume feed should poll sooner: fast=%s slow=%s", fast, slow)
	}
	if failed < 4*time.Hour || failures != 3 {
		t.Fatalf("failure backoff not applied: delay=%s failures=%d", failed, failures)
	}
}

func TestTenantFoldersAndSettingsStayIsolated(t *testing.T) {
	store, db := openTestStore(t)
	ctx := context.Background()
	first := insertTestUser(t, db, "first")
	second := insertTestUser(t, db, "second")
	folder, err := store.CreateFolder(ctx, first, "News")
	if err != nil {
		t.Fatal(err)
	}
	firstFolders, err := store.ListFolders(ctx, first)
	if err != nil || len(firstFolders) != 1 || firstFolders[0].ID != folder.ID {
		t.Fatalf("first folders=%v err=%v", firstFolders, err)
	}
	secondFolders, err := store.ListFolders(ctx, second)
	if err != nil || len(secondFolders) != 0 {
		t.Fatalf("folder leaked to second user: %v err=%v", secondFolders, err)
	}
	updated, err := store.UpdateSettings(ctx, first, map[string]any{"theme": "dark", "markReadOnOpen": false})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Theme != "dark" || updated.MarkReadOnOpen {
		t.Fatalf("settings were not updated: %+v", updated)
	}
	other, err := store.GetSettings(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if other.Theme != "system" || !other.MarkReadOnOpen {
		t.Fatalf("settings leaked to second user: %+v", other)
	}
}
