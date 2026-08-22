package crawl

import "strings"

type FetchClass struct {
	Retryable bool
	Invalid   bool
	Message   string
}

func ClassifyFetch(status int, err error, body, contentType string) FetchClass {
	if status >= 400 && status <= 599 {
		msg := "http error"
		if err != nil {
			msg = err.Error()
		}
		return FetchClass{Retryable: false, Message: msg}
	}
	if status == 0 && err != nil {
		return FetchClass{Retryable: true, Message: err.Error()}
	}
	if strings.TrimSpace(body) == "" {
		return FetchClass{Retryable: false, Invalid: true, Message: "empty page"}
	}
	if !looksLikeHTML(body, contentType) {
		return FetchClass{Retryable: false, Invalid: true, Message: "invalid page"}
	}
	return FetchClass{}
}

func looksLikeHTML(body, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "json") || strings.Contains(ct, "image/") ||
		strings.Contains(ct, "pdf") || strings.Contains(ct, "octet-stream") {
		return false
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "<html") || strings.Contains(lower, "<body") ||
		strings.Contains(lower, "<article") || strings.Contains(lower, "<p") ||
		strings.Contains(lower, "<div")
}
