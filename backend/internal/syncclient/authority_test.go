package syncclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestEventStreamDoesNotInheritRPCTimeout(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/events.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deadlineSeen := false
	client := New(db, Config{
		ServerURL: "http://rss.test", Username: "reader",
		SessionToken: "saved-token", SessionExpiresAt: time.Now().Add(time.Hour),
	}, nil)
	client.HTTP = &http.Client{
		Timeout: time.Nanosecond,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			_, deadlineSeen = req.Context().Deadline()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}),
	}
	if err := client.StreamEvents(context.Background(), func(string, any) {}); err != nil {
		t.Fatal(err)
	}
	if deadlineSeen {
		t.Fatal("event stream inherited the ordinary RPC client timeout")
	}
}

func TestEventStreamAcceptsPayloadLargerThanScannerDefault(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/large-events.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	payload := strings.Repeat("x", 128*1024)
	event := `data: {"event":"sports.cache.updated","payload":{"detail":"` + payload + `"}}` + "\n\n"
	client := New(db, Config{
		ServerURL: "http://rss.test", Username: "reader",
		SessionToken: "saved-token", SessionExpiresAt: time.Now().Add(time.Hour),
	}, nil)
	client.HTTP = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(event)),
				Request:    req,
			}, nil
		}),
	}
	var received string
	if err := client.StreamEvents(context.Background(), func(name string, value any) {
		if name != "sports.cache.updated" {
			t.Fatalf("event name %q", name)
		}
		body, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("payload type %T", value)
		}
		received, _ = body["detail"].(string)
	}); err != nil {
		t.Fatal(err)
	}
	if received != payload {
		t.Fatalf("received payload length %d, want %d", len(received), len(payload))
	}
}
