package serverstore

import (
	"context"
	"testing"
	"time"
)

func TestPushWaitsForEveryEnabledFeedInAccount(t *testing.T) {
	s, db := openTestStore(t)
	ctx := context.Background()
	alice := insertTestUser(t, db, "alice")
	bob := insertTestUser(t, db, "bob")
	for i, url := range []string{"https://a.example/feed", "https://b.example/feed", "https://c.example/feed"} {
		owner := alice
		if i == 2 {
			owner = bob
		}
		_, err := s.SyncFeeds(ctx, owner, SyncRequest{Ops: []FeedOp{{OpID: url, DeviceID: "test", LogicalClock: int64(i + 1), FeedURL: url, Present: true}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	var a, b string
	if err := db.SQL.QueryRow(`SELECT id FROM feeds WHERE url=?`, "https://a.example/feed").Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL.QueryRow(`SELECT id FROM feeds WHERE url=?`, "https://b.example/feed").Scan(&b); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(-time.Minute)
	check := func(want bool) {
		t.Helper()
		got, err := s.PushFeedsReady(ctx, alice, since)
		if err != nil || got != want {
			t.Fatalf("ready=%v want=%v err=%v", got, want, err)
		}
	}
	check(false)
	if _, err := db.SQL.Exec(`UPDATE feeds SET last_success_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), a); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err := db.SQL.Exec(`UPDATE feeds SET last_success_at=? WHERE id=?`, since.Add(-time.Second).UTC().Format(time.RFC3339Nano), b); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err := db.SQL.Exec(`UPDATE feeds SET last_success_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), b); err != nil {
		t.Fatal(err)
	}
	check(true) // Bob's unrefreshed feed cannot block Alice.
	if _, err := db.SQL.Exec(`UPDATE user_feeds SET enabled=0 WHERE user_id=? AND feed_id=?`, alice, b); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.Exec(`UPDATE feeds SET last_success_at=NULL WHERE id=?`, b); err != nil {
		t.Fatal(err)
	}
	check(true)
	owners, err := s.PushFeedOwners(ctx, b)
	if err != nil || owners[alice] {
		t.Fatal("disabled feed generated a notification")
	}
	if _, err := db.SQL.Exec(`UPDATE user_feeds SET enabled=1 WHERE user_id=? AND feed_id=?`, alice, b); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err := db.SQL.Exec(`UPDATE user_feeds SET present=0 WHERE user_id=? AND feed_id=?`, alice, b); err != nil {
		t.Fatal(err)
	}
	check(true)
}
