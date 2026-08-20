package crawl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type Service struct {
	Articles domain.ArticleRepository
	Feeds    domain.FeedRepository
	Log      *slog.Logger
	Emit     func(name string, payload any)
	Client   *http.Client

	mu      sync.Mutex
	running bool
}

func New(articles domain.ArticleRepository, feeds domain.FeedRepository, log *slog.Logger) *Service {
	return &Service{
		Articles: articles,
		Feeds:    feeds,
		Log:      log,
		Client: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (s *Service) EnqueueAndKick(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.loop(ctx)
}

func (s *Service) loop(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		arts, err := s.Articles.ListNeedingCrawl(ctx, 5)
		if err != nil || len(arts) == 0 {
			return
		}
		for _, a := range arts {
			_ = s.CrawlOne(ctx, a.ID)
		}
	}
}

func (s *Service) CrawlOne(ctx context.Context, articleID string) error {
	a, err := s.Articles.Get(ctx, articleID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(a.URL) == "" || !strings.HasPrefix(a.URL, "http") {
		_ = s.Articles.SetCrawlResult(ctx, articleID, domain.CrawlFailed, "", "no http url", false)
		return nil
	}
	_ = s.Articles.SetCrawlResult(ctx, articleID, domain.CrawlPending, a.CrawledContent, "", a.CrawlUnreliable)

	html, finalURL, title, err := s.fetchFullPage(ctx, a.URL)
	if err != nil || strings.TrimSpace(html) == "" {
		msg := "empty page"
		if err != nil {
			msg = err.Error()
		}
		_ = s.Articles.SetCrawlResult(ctx, articleID, domain.CrawlFailed, "", msg, a.CrawlUnreliable)
		_ = s.Feeds.RecordCrawlResult(ctx, a.FeedID, true)
		if s.Emit != nil {
			s.Emit("article.updated", map[string]any{"articleId": articleID, "crawlStatus": "failed"})
		}
		return err
	}
	prepared := EnsureBaseHref(html, finalURL)
	if title != "" && (a.Title == "" || a.IsReadLater || a.Title == a.URL) {
		a.Title = title
		_ = s.Articles.Update(ctx, a)
	}
	_ = s.Articles.SetCrawlResult(ctx, articleID, domain.CrawlOK, prepared, "", false)
	_ = s.Feeds.RecordCrawlResult(ctx, a.FeedID, false)
	if s.Emit != nil {
		s.Emit("article.updated", map[string]any{"articleId": articleID, "crawlStatus": "ok"})
	}
	return nil
}

func (s *Service) FetchLive(ctx context.Context, articleID string) (string, error) {
	a, err := s.Articles.Get(ctx, articleID)
	if err != nil {
		return "", err
	}
	html, finalURL, _, err := s.fetchFullPage(ctx, a.URL)
	if err != nil {
		return "", err
	}
	prepared := EnsureBaseHref(html, finalURL)
	_ = s.Articles.SetLiveContent(ctx, articleID, prepared)
	return prepared, nil
}

func (s *Service) fetchFullPage(ctx context.Context, pageURL string) (html string, finalURL string, title string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RSSReader/0.1; +local desktop)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	res, err := s.Client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("%w: status %d", domain.ErrNetwork, res.StatusCode)
	}
	// Save full document HTML (inline CSS/JS included; external assets resolve via <base>).
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", "", "", err
	}
	html = string(raw)
	finalURL = pageURL
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	title = extractTitle(html)
	if strings.TrimSpace(html) == "" {
		return "", finalURL, title, fmt.Errorf("empty body")
	}
	return html, finalURL, title, nil
}

var reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func extractTitle(html string) string {
	m := reTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(m[1]))
}

// EnsureBaseHref injects a <base> so relative CSS/JS/img URLs resolve against the article origin.
func EnsureBaseHref(html, pageURL string) string {
	if _, err := url.Parse(pageURL); err != nil || pageURL == "" {
		return html
	}
	baseTag := `<base href="` + htmlAttrEscape(pageURL) + `">`
	lower := strings.ToLower(html)
	if strings.Contains(lower, "<base") {
		return html
	}
	if idx := strings.Index(lower, "<head"); idx >= 0 {
		gt := strings.Index(html[idx:], ">")
		if gt >= 0 {
			insertAt := idx + gt + 1
			return html[:insertAt] + baseTag + html[insertAt:]
		}
	}
	if idx := strings.Index(lower, "<html"); idx >= 0 {
		gt := strings.Index(html[idx:], ">")
		if gt >= 0 {
			insertAt := idx + gt + 1
			return html[:insertAt] + "<head>" + baseTag + "</head>" + html[insertAt:]
		}
	}
	return "<head>" + baseTag + "</head>" + html
}

func htmlAttrEscape(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `"`, "&quot;", `'`, "&#39;", `<`, "&lt;", `>`, "&gt;")
	return r.Replace(s)
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
