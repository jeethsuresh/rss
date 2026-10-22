package httpapi

import (
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
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticSignInHintAndCallbackFallback(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := automaticLoginClient(t, "https://news.gravitas.cool")
	handler := New(serverstore.New(db), &application.Service{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{Federation: c})
	for _, value := range []string{"", "1", "untrusted"} {
		req := httptest.NewRequest("GET", "/v1/web/config", nil)
		if value != "" {
			req.AddCookie(&http.Cookie{Name: gravitas.SharedSignInCookie, Value: value})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var config struct {
			Auto bool `json:"autoSignIn"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &config); err != nil {
			t.Fatal(err)
		}
		if config.Auto != (value == "1") {
			t.Fatal("incorrect shared hint")
		}
		session := httptest.NewRequest("GET", "/v1/web/session", nil)
		session.Header.Set("Cookie", req.Header.Get("Cookie"))
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, session)
		if w.Code != 401 {
			t.Fatal("marker authenticated an RSS session")
		}
	}
	exerciseAutomaticCallback(t, handler, c.AppURL)
}

type automaticLoginTransport func(*http.Request) (*http.Response, error)

func (f automaticLoginTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func automaticLoginClient(t *testing.T, app string) *gravitas.Client {
	t.Helper()
	c, err := gravitas.New("https://gravitas.cool", app, base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	c.HTTP.Transport = automaticLoginTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://gravitas.cool/.well-known/iam-configuration" {
			t.Fatalf("unexpected IAM request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"issuer":"https://gravitas.cool","identity_domain":"gravitas.cool","browser_initiation_endpoint":"https://gravitas.cool/oauth/start","token_endpoint":"https://gravitas.cool/oauth/token","userinfo_endpoint":"https://gravitas.cool/app/v1/me","session_check_endpoint":"https://gravitas.cool/oauth/session"}`)), Header: make(http.Header)}, nil
	})
	return c
}

func exerciseAutomaticCallback(t *testing.T, handler http.Handler, app string) {
	t.Helper()
	for _, tc := range []struct {
		name, prompt, failure, destination string
		status                             int
	}{
		{"silent login required", "none", "login_required", "/login", 303},
		{"silent consent required", "none", "consent_required", "/auth/gravitas/start", 303},
		{"interactive consent error stops", "", "consent_required", "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := "/#document=keep-this-link"
			initial := httptest.NewRecorder()
			handler.ServeHTTP(initial, httptest.NewRequest("GET", app+"/auth/gravitas/start?"+url.Values{"next": {next}, "prompt": {tc.prompt}}.Encode(), nil))
			if initial.Code != 303 {
				t.Fatalf("start status %d", initial.Code)
			}
			authURL, err := url.Parse(initial.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			state := authURL.Query().Get("state")
			req := httptest.NewRequest("GET", app+"/auth/gravitas/callback?"+url.Values{"state": {state}, "error": {tc.failure}}.Encode(), nil)
			req.AddCookie(initial.Result().Cookies()[0])
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, req)
			if result.Code != tc.status {
				t.Fatalf("callback status %d", result.Code)
			}
			if tc.destination == "" {
				if result.Header().Get("Location") != "" {
					t.Fatal("interactive failure looped")
				}
				return
			}
			location, err := url.Parse(result.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if location.Path != tc.destination || location.Query().Get("next") != next {
				t.Fatalf("lost safe return: %s", location.String())
			}
			if tc.failure == "login_required" && location.Query().Get("auto") != "off" {
				t.Fatal("passive failure can automatically loop")
			}
			if tc.failure == "consent_required" {
				restarted := httptest.NewRecorder()
				handler.ServeHTTP(restarted, httptest.NewRequest("GET", app+location.String(), nil))
				if restarted.Code != 303 {
					t.Fatal("interactive restart failed")
				}
				interactive, err := url.Parse(restarted.Header().Get("Location"))
				if err != nil {
					t.Fatal(err)
				}
				if interactive.Query().Get("prompt") != "" || interactive.Query().Get("state") == state {
					t.Fatal("consent did not start a fresh interactive PKCE flow")
				}
			}
		})
	}
}
