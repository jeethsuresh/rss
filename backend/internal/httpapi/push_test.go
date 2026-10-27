package httpapi

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/push"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushSubscriptionRequiresSessionCSRFAndOwnerAndLogoutRevokes(t *testing.T) {
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	path := filepath.Join(t.TempDir(), "push.json")
	publisher, e := push.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	handler := New(serverstore.New(db), &application.Service{Version: "test"}, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{RegistrationEnabled: true, RegistrationKey: "test-invite", Push: publisher, Context: context.Background()})
	call := func(method, path, body string, cookie *http.Cookie, csrf bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf {
			r.Header.Set("X-RSS-CSRF", "1")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if call("GET", "/v1/push/key", "", nil, false).Code != 401 {
		t.Fatal("anonymous subscription key exposed")
	}
	register := func(name string) *http.Cookie {
		t.Helper()
		res := call("POST", "/v1/web/register", `{"username":"`+name+`","password":"correct horse battery staple","inviteKey":"test-invite"}`, nil, true)
		if res.Code != 201 {
			t.Fatal(res.Body.String())
		}
		return res.Result().Cookies()[0]
	}
	alice, bob := register("alice"), register("bob")
	key := call("GET", "/v1/push/key", "", alice, false)
	var config map[string]string
	_ = json.Unmarshal(key.Body.Bytes(), &config)
	if key.Code != 200 || config["owner"] == "" || config["publicKey"] == "" || key.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing account-scoped push configuration")
	}
	private, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	body := `{"endpoint":"https://fcm.googleapis.com/test","keys":{"auth":"` + base64.RawURLEncoding.EncodeToString(auth) + `","p256dh":"` + base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()) + `"}}`
	if call("POST", "/v1/push/subscription", body, alice, false).Code != 403 || call("POST", "/v1/push/subscription", body, alice, true).Code != 200 {
		t.Fatal("subscription did not enforce authenticated request verification")
	}
	count := func() int {
		t.Helper()
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var state struct {
			Devices map[string]push.Device `json:"devices"`
		}
		if e = json.Unmarshal(data, &state); e != nil {
			t.Fatal(e)
		}
		return len(state.Devices)
	}
	if call("DELETE", "/v1/push/subscription", `{"endpoint":"https://fcm.googleapis.com/test"}`, bob, true).Code != 200 || count() != 1 {
		t.Fatal("another account removed the subscription")
	}
	if call("POST", "/v1/web/logout", `{}`, alice, true).Code != 200 || count() != 0 {
		t.Fatal("sign-out retained a background grant")
	}
	_ = publisher.Notify("news", "", map[string]bool{config["owner"]: true}, push.Notification{})
	publisher.Dispatch(context.Background(), publisher.Now())
}
