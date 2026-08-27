package syncclient

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
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
