package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestRegistrationInviteOnEveryPublicPath(t *testing.T) {
	for _, endpoint := range []string{"auth", "web"} {
		for _, tc := range []struct {
			name, configured, provided string
			enabled                    bool
			status                     int
		}{
			{"missing", "test-invite", "", true, http.StatusForbidden},
			{"wrong", "test-invite", "wrong-invite", true, http.StatusForbidden},
			{"correct", "test-invite", "test-invite", true, http.StatusCreated},
			{"unconfigured", "", "", true, http.StatusForbidden},
			{"disabled", "test-invite", "test-invite", false, http.StatusForbidden},
		} {
			t.Run(endpoint+"/"+tc.name, func(t *testing.T) {
				db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				handler := New(serverstore.New(db), &application.Service{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{RegistrationEnabled: tc.enabled, RegistrationKey: tc.configured, Context: context.Background()})
				body, _ := json.Marshal(map[string]string{"username": "invited-reader", "password": "correct horse battery staple", "inviteKey": tc.provided})
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/"+endpoint+"/register", string(body)))
				if response.Code != tc.status {
					t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
				var users int
				if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
					t.Fatal(err)
				}
				if tc.status != http.StatusCreated && users != 0 {
					t.Fatal("rejected registration created a user")
				}
				if tc.status == http.StatusCreated {
					login := httptest.NewRecorder()
					handler.ServeHTTP(login, jsonRequest(http.MethodPost, "/v1/"+endpoint+"/login", `{"username":"invited-reader","password":"correct horse battery staple"}`))
					if login.Code != http.StatusOK {
						t.Fatalf("ordinary login must not require an invite: %d", login.Code)
					}
				}
				config := httptest.NewRecorder()
				handler.ServeHTTP(config, httptest.NewRequest(http.MethodGet, "/v1/web/config", nil))
				if tc.configured != "" && strings.Contains(config.Body.String(), tc.configured) {
					t.Fatal("web configuration exposed the invite key")
				}
				var publicConfig struct {
					RegistrationEnabled bool `json:"registrationEnabled"`
				}
				if err := json.Unmarshal(config.Body.Bytes(), &publicConfig); err != nil {
					t.Fatal(err)
				}
				if publicConfig.RegistrationEnabled != (tc.enabled && tc.configured != "") {
					t.Fatal("incorrect public registration state")
				}
			})
		}
	}
}
