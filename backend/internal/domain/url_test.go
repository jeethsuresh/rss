package domain

import (
	"errors"
	"testing"
)

func TestNormalizeReadLaterURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https URL", raw: "https://example.com/article?q=1", want: "https://example.com/article?q=1"},
		{name: "http URL", raw: "http://example.com", want: "http://example.com"},
		{name: "scheme-less host", raw: "example.com/article", want: "https://example.com/article"},
		{name: "trims whitespace", raw: "  https://example.com/article  ", want: "https://example.com/article"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeReadLaterURL(tt.raw)
			if err != nil {
				t.Fatalf("NormalizeReadLaterURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeReadLaterURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeReadLaterURLRejectsInvalidInput(t *testing.T) {
	invalid := []string{
		"",
		"   ",
		"not a url at all",
		"https:///missing-host",
		"ftp://example.com/file",
		"javascript:alert(1)",
		"https://example.com/%zz",
	}

	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			_, err := NormalizeReadLaterURL(raw)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("NormalizeReadLaterURL(%q) error = %v, want ErrInvalidURL", raw, err)
			}
		})
	}
}
