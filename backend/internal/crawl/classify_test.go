package crawl

import (
	"errors"
	"testing"
)

func TestClassifyTimeoutIsRetryable(t *testing.T) {
	got := ClassifyFetch(0, errors.New("context deadline exceeded"), "", "")
	if !got.Retryable || got.Invalid {
		t.Fatalf("got %+v", got)
	}
}

func TestClassify404IsNotRetryable(t *testing.T) {
	got := ClassifyFetch(404, errors.New("status 404"), "<html>missing</html>", "text/html")
	if got.Retryable {
		t.Fatalf("404 should not retry, got %+v", got)
	}
}

func TestClassify500IsNotRetryable(t *testing.T) {
	got := ClassifyFetch(500, errors.New("status 500"), "oops", "text/plain")
	if got.Retryable {
		t.Fatalf("500 should not retry, got %+v", got)
	}
}

func TestClassifyEmpty200IsInvalid(t *testing.T) {
	got := ClassifyFetch(200, nil, "  ", "text/html")
	if got.Retryable || !got.Invalid {
		t.Fatalf("empty page should be invalid and not retryable, got %+v", got)
	}
}

func TestClassifyJSONIsInvalid(t *testing.T) {
	got := ClassifyFetch(200, nil, `{"ok":true}`, "application/json")
	if got.Retryable || !got.Invalid {
		t.Fatalf("json should be invalid, got %+v", got)
	}
}

func TestClassifyHTML200IsValid(t *testing.T) {
	got := ClassifyFetch(200, nil, "<html><body><p>hi</p></body></html>", "text/html; charset=utf-8")
	if got.Invalid || got.Retryable {
		t.Fatalf("html 200 should be usable, got %+v", got)
	}
}
