package serverfetch

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/crawl"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

var documentTitlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

type DocumentScheduler struct {
	Store  *serverstore.Store
	Log    *slog.Logger
	Client *http.Client
}

func NewDocumentScheduler(store *serverstore.Store, log *slog.Logger) *DocumentScheduler {
	return &DocumentScheduler{
		Store: store,
		Log:   log,
		Client: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (s *DocumentScheduler) Run(ctx context.Context) {
	owner := uuid.NewString()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		s.runOne(ctx, owner)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *DocumentScheduler) runOne(ctx context.Context, owner string) {
	claim, err := s.Store.ClaimDueDocument(ctx, owner, 2*time.Minute)
	if err != nil {
		s.Log.Error("claim read-later document", "err", err)
		return
	}
	if claim == nil {
		return
	}
	finalURL, title, body, reader, fetchErr := s.fetch(ctx, claim.URL)
	if err := s.Store.CompleteDocument(ctx, *claim, finalURL, title, body, reader, fetchErr); err != nil {
		s.Log.Error("complete read-later document", "documentId", claim.DocumentID, "err", err)
	}
	if fetchErr != nil {
		s.Log.Warn("fetch read-later document", "documentId", claim.DocumentID, "err", fetchErr)
	}
}

func (s *DocumentScheduler) fetch(ctx context.Context, pageURL string) (finalURL, title, body, reader string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("User-Agent", "RSSReader-Server/0.2")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	res, err := s.Client.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		return "", "", "", "", fmt.Errorf("status %d", res.StatusCode)
	}
	contentType := strings.ToLower(res.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "html") && !strings.Contains(contentType, "xml") {
		return "", "", "", "", fmt.Errorf("unsupported content type %s", contentType)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", "", "", "", err
	}
	body = string(raw)
	if strings.TrimSpace(body) == "" {
		return "", "", "", "", fmt.Errorf("empty response")
	}
	finalURL = pageURL
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	body = crawl.EnsureBaseHref(body, finalURL)
	if match := documentTitlePattern.FindStringSubmatch(body); len(match) == 2 {
		title = strings.TrimSpace(html.UnescapeString(stripTags(match[1])))
	}
	reader, _ = crawl.ExtractHTML(body)
	return finalURL, title, body, reader, nil
}

func stripTags(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
