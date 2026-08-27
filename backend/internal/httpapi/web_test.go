package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestWebLoginGateAndRPC(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><title>web app</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := New(
		serverstore.New(db),
		&application.Service{Version: "test"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config{RegistrationEnabled: true, Version: "test", Context: context.Background(), WebDir: webDir},
	)

	root := httptest.NewRecorder()
	handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusSeeOther || root.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated root: status=%d location=%q", root.Code, root.Header().Get("Location"))
	}
	traversal := httptest.NewRecorder()
	traversalRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	traversalRequest.URL.Path = "/../go.mod"
	handler.ServeHTTP(traversal, traversalRequest)
	if traversal.Code == http.StatusOK {
		t.Fatalf("static file traversal unexpectedly succeeded: %s", traversal.Body.String())
	}

	register := httptest.NewRecorder()
	handler.ServeHTTP(register, jsonRequest(http.MethodPost, "/v1/web/register", `{"username":"reader","password":"correct horse battery staple"}`))
	if register.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", register.Code, register.Body.String())
	}
	if strings.Contains(register.Body.String(), "token") {
		t.Fatalf("web registration exposed bearer token: %s", register.Body.String())
	}
	cookies := register.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != webSessionCookie || !cookies[0].HttpOnly {
		t.Fatalf("missing secure web session cookie: %#v", cookies)
	}

	rpc := httptest.NewRecorder()
	rpcRequest := jsonRequest(http.MethodPost, "/v1/rpc", `{"method":"feeds.list","params":{}}`)
	rpcRequest.AddCookie(cookies[0])
	rpcRequest.Header.Set("X-RSS-CSRF", "1")
	handler.ServeHTTP(rpc, rpcRequest)
	var feeds []domain.Feed
	if err := json.Unmarshal(rpc.Body.Bytes(), &feeds); err != nil {
		t.Fatal(err)
	}
	if rpc.Code != http.StatusOK || len(feeds) != 1 || !feeds[0].IsReadLater || feeds[0].UnreadCount != 0 {
		t.Fatalf("rpc: status=%d body=%s", rpc.Code, rpc.Body.String())
	}

	app := httptest.NewRecorder()
	appRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	appRequest.AddCookie(cookies[0])
	handler.ServeHTTP(app, appRequest)
	if app.Code != http.StatusOK || !strings.Contains(app.Body.String(), "web app") {
		t.Fatalf("authenticated app: status=%d body=%s", app.Code, app.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	badRequest := jsonRequest(http.MethodPost, "/v1/rpc", `{"method":"system.ping","params":{}}`)
	badRequest.AddCookie(cookies[0])
	handler.ServeHTTP(missingCSRF, badRequest)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF header: status=%d", missingCSRF.Code)
	}
}

func jsonRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}
