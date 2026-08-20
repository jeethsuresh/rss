package rss_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/rss"
)

func TestFingerprintPrefersGUID(t *testing.T) {
	fp := rss.Fingerprint("abc", "https://example.com/a", "Title", nil)
	if fp != "guid:abc" {
		t.Fatalf("got %q", fp)
	}
}

func TestFingerprintFallsBackToURL(t *testing.T) {
	fp := rss.Fingerprint("", "https://example.com/a", "Title", nil)
	if fp != "url:https://example.com/a" {
		t.Fatalf("got %q", fp)
	}
}

func TestFingerprintHashWithoutIDs(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	fp := rss.Fingerprint("", "", "Hello", &now)
	if !strings.HasPrefix(fp, "fp:") || len(fp) < 10 {
		t.Fatalf("unexpected fingerprint %q", fp)
	}
}

func TestNormalizeArticles(t *testing.T) {
	// Use parser via Fetch is integration; unit-test fingerprint path via Normalize with gofeed.Feed
}
