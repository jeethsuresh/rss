package domain

import (
	"net/url"
	"strings"
)

// NormalizeReadLaterURL accepts HTTP(S) URLs and scheme-less hosts.
// Scheme-less values are stored as HTTPS URLs.
func NormalizeReadLaterURL(rawURL string) (string, error) {
	candidate := strings.TrimSpace(rawURL)
	if candidate == "" {
		return "", ErrInvalidURL
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", ErrInvalidURL
	}
	if parsed.Scheme == "" {
		candidate = "https://" + candidate
		parsed, err = url.Parse(candidate)
		if err != nil {
			return "", ErrInvalidURL
		}
	}

	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", ErrInvalidURL
	}
	if parsed.Hostname() == "" {
		return "", ErrInvalidURL
	}

	return candidate, nil
}
