package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/gravitas"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFederationDisablesEveryLocalLoginAndSignup(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := serverstore.New(db)
	local, err := store.Register(context.Background(), "old-reader", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	c, err := gravitas.New("https://gravitas.cool", "https://news.gravitas.cool", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(store, &application.Service{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{Federation: c, RegistrationEnabled: true, RegistrationKey: "old-invite"})
	for _, path := range []string{"/v1/auth/login", "/v1/web/login"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, jsonRequest("POST", path, `{"username":"old-reader","password":"correct horse battery staple"}`))
		if w.Code != 401 {
			t.Fatalf("local login bypass: %s %d", path, w.Code)
		}
	}
	for _, path := range []string{"/v1/auth/register", "/v1/web/register"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, jsonRequest("POST", path, `{"username":"new-reader","password":"correct horse battery staple","inviteKey":"old-invite"}`))
		if w.Code != 403 {
			t.Fatalf("local signup bypass: %s %d", path, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/v1/web/session", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-rss_session", Value: local.Token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("old local session bypassed IAM")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/v1/web/config", nil))
	var config struct {
		Federated    bool `json:"federated"`
		Registration bool `json:"registrationEnabled"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &config); err != nil || !config.Federated || config.Registration {
		t.Fatal("incorrect hosted auth configuration")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/auth/gravitas/ready.json", nil))
	if w.Code != 200 {
		t.Fatal("federated readiness failed")
	}
}
