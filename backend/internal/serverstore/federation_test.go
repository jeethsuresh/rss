package serverstore

import (
	"context"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
	"path/filepath"
	"testing"
)

func TestFederatedIdentityIsStableAndCannotClaimLocalUsername(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	local, err := s.Register(ctx, "alice", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.LoginFederated(ctx, "https://gravitas.cool", "subject-1", "@alice:gravitas.cool")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.LoginFederated(ctx, "https://gravitas.cool", "subject-1", "@renamed:gravitas.cool")
	if err != nil {
		t.Fatal(err)
	}
	if first.User.ID == local.User.ID || first.User.ID != again.User.ID || again.User.DisplayName != "@renamed:gravitas.cool" {
		t.Fatal("identity mapping used a mutable name")
	}
	other, err := s.LoginFederated(ctx, "https://other.example", "subject-1", "@alice:other.example")
	if err != nil {
		t.Fatal(err)
	}
	if other.User.ID == first.User.ID {
		t.Fatal("issuer was ignored")
	}
	if _, err = s.AuthenticateFederated(ctx, local.Token, "https://gravitas.cool"); err == nil {
		t.Fatal("local password session bypassed IAM")
	}
	if _, err = s.AuthenticateFederated(ctx, first.Token, "https://other.example"); err == nil {
		t.Fatal("foreign issuer session accepted")
	}
	if _, err = s.AuthenticateFederated(ctx, first.Token, "https://gravitas.cool"); err != nil {
		t.Fatal(err)
	}
	if first.ExpiresAt.Sub(s.now()).Hours() > 8.01 {
		t.Fatal("SSO session outlives policy")
	}
}
func TestFederatedAccountKeepsExistingLocalDataSeparate(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	local, err := s.Register(ctx, "existing-reader", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.SQL.Exec(`INSERT INTO user_folders(id,user_id,name,created_at) VALUES ('kept-folder',?,'Saved folder','2026-01-01T00:00:00Z')`, local.User.ID); err != nil {
		t.Fatal(err)
	}
	linked, err := s.LoginFederated(ctx, "https://gravitas.cool", "subject-1", "@admin:gravitas.cool")
	if err != nil {
		t.Fatal(err)
	}
	if linked.User.ID == local.User.ID {
		t.Fatal("local account was linked without permission")
	}
	var count int
	if err = db.SQL.QueryRow(`SELECT COUNT(*) FROM user_folders WHERE user_id=?`, local.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("existing data changed")
	}
	var fresh int
	if err = db.SQL.QueryRow(`SELECT COUNT(*) FROM user_folders WHERE user_id=?`, linked.User.ID).Scan(&fresh); err != nil || fresh != 0 {
		t.Fatal("local data leaked into the IAM account")
	}
}
