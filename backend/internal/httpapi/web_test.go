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
	"time"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/cluster"
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
		Config{RegistrationEnabled: true, RegistrationKey: "test-invite", Version: "test", Context: context.Background(), WebDir: webDir},
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
	handler.ServeHTTP(register, jsonRequest(http.MethodPost, "/v1/web/register", `{"username":"reader","password":"correct horse battery staple","inviteKey":"test-invite"}`))
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

func TestWebCannotChangeAISettings(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settingsRepo := sqlite.NewSettingsRepo(db)
	current, err := settingsRepo.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	current.AIEnabled = false
	current.AIBaseURL = "https://ai.example/v1"
	current.AIModel = "server-model"
	if err := settingsRepo.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	handler := New(
		serverstore.New(db),
		&application.Service{Settings: settingsRepo, Version: "test"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config{RegistrationEnabled: true, RegistrationKey: "test-invite", Version: "test", Context: context.Background()},
	)

	register := httptest.NewRecorder()
	handler.ServeHTTP(register, jsonRequest(http.MethodPost, "/v1/web/register", `{"username":"reader","password":"correct horse battery staple","inviteKey":"test-invite"}`))
	if register.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", register.Code, register.Body.String())
	}
	cookie := register.Result().Cookies()[0]

	update := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/rpc", `{"method":"settings.update","params":{"aiEnabled":true,"aiBaseUrl":"http://evil.example/v1","aiModel":"stolen","theme":"dark"}}`)
	request.AddCookie(cookie)
	request.Header.Set("X-RSS-CSRF", "1")
	handler.ServeHTTP(update, request)
	if update.Code != http.StatusOK {
		t.Fatalf("settings.update: status=%d body=%s", update.Code, update.Body.String())
	}
	var settings domain.Settings
	if err := json.Unmarshal(update.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.AIEnabled || settings.AIBaseURL != "" || settings.AIModel != "" || settings.Theme != "dark" {
		t.Fatalf("web settings = %#v", settings)
	}
	var enabled int
	var base, model string
	if err := db.SQL.QueryRow(`SELECT ai_enabled, ai_base_url, ai_model FROM settings WHERE id=1`).Scan(&enabled, &base, &model); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 || base != "https://ai.example/v1" || model != "server-model" {
		t.Fatalf("stored AI config = enabled:%d base:%q model:%q", enabled, base, model)
	}
}

func jsonRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// Exercise the same authenticated RPC path the hosted reader uses, including
// splits that replace an umbrella with new groups or only individual articles.
func TestWebStorySplit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pairs      bool
		wantGroups int
	}{{"replacement groups", true, 2}, {"individual articles", false, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			store := serverstore.New(db)
			articles := sqlite.NewArticleRepo(db)
			stories := sqlite.NewStoryRepo(db)
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := &application.Service{Articles: articles, Stories: stories, Cluster: cluster.New(articles, stories, log)}
			handler := New(store, svc, log, Config{RegistrationEnabled: true})
			register := httptest.NewRecorder()
			handler.ServeHTTP(register, jsonRequest(http.MethodPost, "/v1/web/register", `{"username":"reader","password":"correct horse battery staple","inviteKey":"test-invite"}`))
			if register.Code != http.StatusCreated {
				t.Fatalf("register: %s", register.Body.String())
			}
			var session struct {
				User serverstore.User `json:"user"`
			}
			if err := json.Unmarshal(register.Body.Bytes(), &session); err != nil {
				t.Fatal(err)
			}
			feed, err := store.SetFeedMembership(ctx, session.User.ID, "https://example.com/feed", true)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			members := []domain.Article{
				{ID: "kyiv-a", Title: "Biden in Kyiv", Summary: "President Biden meets Zelensky in Kyiv after the strike."},
				{ID: "apple-a", Title: "Apple visor", Summary: "Apple unveils a new headset in Cupertino today."},
			}
			if tc.pairs {
				members = append(members,
					domain.Article{ID: "kyiv-b", Title: members[0].Title, Summary: members[0].Summary},
					domain.Article{ID: "apple-b", Title: members[1].Title, Summary: members[1].Summary})
			}
			ids := make([]string, 0, len(members))
			for i := range members {
				members[i].FeedID = feed.ID
				members[i].URL = "https://example.com/" + members[i].ID
				members[i].ExternalID = members[i].ID
				members[i].RSSContent = members[i].Summary
				members[i].DiscoveredAt = now
				ids = append(ids, members[i].ID)
			}
			if _, err := articles.UpsertMany(ctx, members); err != nil {
				t.Fatal(err)
			}
			if err := stories.Create(ctx, &domain.Story{ID: "umbrella", Title: "Mixed coverage", CreatedAt: now, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if err := stories.SetMembers(ctx, "umbrella", ids); err != nil {
				t.Fatal(err)
			}
			read := true
			if _, err := store.SetArticleState(ctx, session.User.ID, ids[0], serverstore.ArticleStatePatch{IsRead: &read}); err != nil {
				t.Fatal(err)
			}
			rpc := func(method string, params any) *httptest.ResponseRecorder {
				t.Helper()
				body, err := json.Marshal(map[string]any{"method": method, "params": params})
				if err != nil {
					t.Fatal(err)
				}
				request := jsonRequest(http.MethodPost, "/v1/rpc", string(body))
				request.AddCookie(register.Result().Cookies()[0])
				request.Header.Set("X-RSS-CSRF", "1")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", method, response.Code, response.Body.String())
				}
				return response
			}
			var split struct {
				StoryIDs []string `json:"storyIds"`
			}
			if err := json.Unmarshal(rpc("stories.split", map[string]string{"id": "umbrella"}).Body.Bytes(), &split); err != nil {
				t.Fatal(err)
			}
			if len(split.StoryIDs) != tc.wantGroups {
				t.Fatalf("split IDs: %v", split.StoryIDs)
			}
			var list []domain.Story
			if err := json.Unmarshal(rpc("stories.list", map[string]any{}).Body.Bytes(), &list); err != nil {
				t.Fatal(err)
			}
			if len(list) != tc.wantGroups {
				t.Fatalf("visible groups: %v", list)
			}
			for _, id := range split.StoryIDs {
				var child domain.Story
				if err := json.Unmarshal(rpc("stories.get", map[string]string{"id": id}).Body.Bytes(), &child); err != nil {
					t.Fatal(err)
				}
				if child.MemberCount != 2 || len(child.Articles) != 2 {
					t.Fatalf("child coverage: %+v", child)
				}
			}
			old, err := stories.Get(ctx, "umbrella")
			if err != nil || old.MemberCount != 0 {
				t.Fatalf("old umbrella retained members: %+v %v", old, err)
			}
			original, err := store.GetArticle(ctx, session.User.ID, ids[0])
			if err != nil || !original.IsRead {
				t.Fatalf("split lost user read state: %+v %v", original, err)
			}
		})
	}
}
