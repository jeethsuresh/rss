package syncclient

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestLoginRetriesHTTPPostOnSameHostHTTPSUpgrade(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var calls []string
	client := New(db, Config{
		ServerURL: "http://rss.jeeth.dev", Username: "reader", Password: "secret",
	}, slog.Default())
	client.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.String())
		switch req.URL.String() {
		case "http://rss.jeeth.dev/v1/auth/login":
			if req.Method != http.MethodPost {
				t.Errorf("login method = %s", req.Method)
			}
			return httpResponse(req, http.StatusMovedPermanently, "<html>301</html>", "https://rss.jeeth.dev/v1/auth/login"), nil
		case "https://rss.jeeth.dev/v1/auth/login":
			if req.Method != http.MethodPost {
				t.Errorf("retried login method = %s", req.Method)
			}
			return httpResponse(req, http.StatusOK, `{"token":"abc","expiresAt":"2030-01-01T00:00:00Z"}`, ""), nil
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL)
			return httpResponse(req, http.StatusBadGateway, "unexpected", ""), nil
		}
	})}

	token, err := client.login(context.Background(), "http://rss.jeeth.dev")
	if err != nil {
		t.Fatal(err)
	}
	if token != "abc" {
		t.Fatalf("token = %q", token)
	}
	if client.baseURL() != "https://rss.jeeth.dev" {
		t.Fatalf("server URL = %q", client.baseURL())
	}
	if strings.Join(calls, "\n") != strings.Join([]string{
		"POST http://rss.jeeth.dev/v1/auth/login",
		"POST https://rss.jeeth.dev/v1/auth/login",
	}, "\n") {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestLoginRejectsCrossHostRedirect(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var calls []string
	client := New(db, Config{
		ServerURL: "http://rss.jeeth.dev", Username: "reader", Password: "secret",
	}, slog.Default())
	client.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.String())
		return httpResponse(req, http.StatusMovedPermanently, "<html>301</html>", "https://evil.example/v1/auth/login"), nil
	})}

	_, err = client.login(context.Background(), "http://rss.jeeth.dev")
	if err == nil || !strings.Contains(err.Error(), "https://evil.example/v1/auth/login") {
		t.Fatalf("error = %v", err)
	}
	if strings.Join(calls, ",") != "http://rss.jeeth.dev/v1/auth/login" {
		t.Fatalf("followed redirect: %#v", calls)
	}
	if client.baseURL() != "http://rss.jeeth.dev" {
		t.Fatalf("server URL = %q", client.baseURL())
	}
}

func TestDesktopAIStaysLocal(t *testing.T) {
	if IsAuthorityMethod("ai.test") || IsAuthorityMethod("ai.scan") || IsAuthorityMutation("ai.scan") {
		t.Fatal("AI methods are handled by the desktop, not the sync server")
	}
	if !IsAuthorityMethod("settings.update") || !IsAuthorityMutation("settings.update") {
		t.Fatal("reader settings still sync")
	}
}

func httpResponse(req *http.Request, status int, body, location string) *http.Response {
	header := make(http.Header)
	if location != "" {
		header.Set("Location", location)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
