package httpapi

import (
	"bufio"
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
	"strings"
	"sync"
	"testing"
	"time"
)

type rssAuthority struct {
	mu      sync.Mutex
	auth    map[string]gravitas.Authorization
	revoked map[string]bool
	failure int
}

func (a *rssAuthority) RoundTrip(r *http.Request) (*http.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	code := 200
	var body any
	if r.URL.Path == "/.well-known/iam-configuration" {
		body = map[string]string{"issuer": "https://gravitas.cool", "identity_domain": "gravitas.cool", "browser_initiation_endpoint": "https://gravitas.cool/oauth/start", "token_endpoint": "https://gravitas.cool/oauth/token", "userinfo_endpoint": "https://gravitas.cool/app/v1/me", "session_check_endpoint": "https://gravitas.cool/oauth/session"}
	} else {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		auth, ok := a.auth[token]
		if a.failure != 0 {
			code = a.failure
		} else if !ok || a.revoked[token] {
			code = 401
		}
		body = map[string]any{"active": true, "iss": auth.Issuer, "sub": auth.Subject, "app_origin": "https://news.gravitas.cool", "session_id": auth.SessionID, "exp": auth.CheckExpires}
	}
	raw, _ := json.Marshal(body)
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(string(raw))), Header: make(http.Header)}, nil
}
func rssBoundFixture(t *testing.T) (http.Handler, *gravitas.Client, gravitas.Authorization, *rssAuthority, *serverstore.Store, *EventHub) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	c, err := gravitas.New("https://gravitas.cool", "https://news.gravitas.cool", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	auth := gravitas.Authorization{Identity: gravitas.Identity{Issuer: c.Issuer, Subject: "subject-a", Username: "reader", Domain: "gravitas.cool", UserID: "@reader:gravitas.cool"}, CheckToken: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), SessionID: "11111111-1111-4111-8111-111111111111", CheckExpires: time.Now().Add(time.Hour).Unix()}
	authority := &rssAuthority{auth: map[string]gravitas.Authorization{auth.CheckToken: auth}, revoked: map[string]bool{}}
	c.HTTP.Transport = authority
	st := serverstore.New(db)
	hub := NewEventHub()
	handler := New(st, &application.Service{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{Federation: c, Events: hub})
	return handler, c, auth, authority, st, hub
}
func rssCookie(t *testing.T, c *gravitas.Client, st *serverstore.Store, auth gravitas.Authorization) (*http.Cookie, string) {
	t.Helper()
	result, err := st.LoginFederated(context.Background(), auth.Issuer, auth.Subject, auth.UserID)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if err = c.SetAppSession(w, "__Host-rss_session", auth, result.Token, result.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	return w.Result().Cookies()[0], result.Token
}
func TestRSSFamilyRevocationOutageLegacyAndLocalLogout(t *testing.T) {
	handler, c, auth, authority, st, _ := rssBoundFixture(t)
	cookie, localToken := rssCookie(t, c, st, auth)
	request := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/v1/web/session", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request(cookie); w.Code != 200 {
		t.Fatalf("bound session denied %d %s", w.Code, w.Body.String())
	}
	if w := request(&http.Cookie{Name: cookie.Name, Value: localToken}); w.Code != 401 {
		t.Fatal("legacy opaque token bypassed proof")
	}
	other := auth
	other.Subject = "subject-b"
	other.CheckToken = base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	authority.auth[other.CheckToken] = other
	w := httptest.NewRecorder()
	if err := c.SetAppSession(w, cookie.Name, other, localToken, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if request(w.Result().Cookies()[0]).Code != 401 {
		t.Fatal("mismatched local tenant accepted")
	}
	authB := auth
	authB.CheckToken = base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("c", 32)))
	authB.SessionID = "22222222-2222-4222-8222-222222222222"
	authority.auth[authB.CheckToken] = authB
	cookieB, _ := rssCookie(t, c, st, authB)
	authority.failure = 503
	if w := request(cookie); w.Code != 503 || len(w.Result().Cookies()) != 0 {
		t.Fatal("outage permitted access or cleared cookie")
	}
	authority.failure = 0
	if request(cookie).Code != 200 {
		t.Fatal("outage destroyed local session")
	}
	authority.revoked[auth.CheckToken] = true
	if w := request(cookie); w.Code != 401 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("revoked family accepted")
	}
	if request(cookieB).Code != 200 {
		t.Fatal("independent family revoked")
	}
	authority.failure = 503
	r := httptest.NewRequest("POST", "/v1/web/logout", nil)
	r.Header.Set("X-RSS-CSRF", "1")
	r.AddCookie(cookieB)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("local logout requires IAM availability")
	}
	authority.failure = 0
	if request(cookieB).Code != 401 {
		t.Fatal("locally logged out token retained access")
	}
}
func TestRSSEventStreamStopsBeforePostRevocationEvent(t *testing.T) {
	handler, c, auth, authority, st, hub := rssBoundFixture(t)
	cookie, _ := rssCookie(t, c, st, auth)
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/v1/events", nil)
	r.AddCookie(cookie)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	if _, err = reader.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	authority.mu.Lock()
	authority.revoked[auth.CheckToken] = true
	authority.mu.Unlock()
	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.Lock()
		ready := len(hub.subscribers) > 0
		hub.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream never subscribed")
		}
		time.Sleep(time.Millisecond)
	}
	hub.Emit("test.private", map[string]string{"secret": "MUST_NOT_LEAK"})
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "event: auth-required") || strings.Contains(string(body), "MUST_NOT_LEAK") {
		t.Fatalf("revoked stream leaked data: %s", body)
	}
}
