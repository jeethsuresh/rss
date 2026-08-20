package crawl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

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
			Timeout: 45 * time.Second,
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

	text, title, err := s.fetchReadable(ctx, a.URL)
	failed := err != nil || strings.TrimSpace(text) == ""
	if failed {
		msg := "empty extract"
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
	if title != "" && (a.Title == "" || a.IsReadLater) {
		a.Title = title
		_ = s.Articles.Update(ctx, a)
	}
	_ = s.Articles.SetCrawlResult(ctx, articleID, domain.CrawlOK, text, "", false)
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
	text, _, err := s.fetchReadable(ctx, a.URL)
	if err != nil {
		return "", err
	}
	_ = s.Articles.SetLiveContent(ctx, articleID, text)
	return text, nil
}

func (s *Service) fetchReadable(ctx context.Context, pageURL string) (body string, title string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; readability)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	res, err := s.Client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", fmt.Errorf("%w: status %d", domain.ErrNetwork, res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", "", err
	}
	html := string(raw)
	title = extractTitle(html)
	text := extractMain(html)
	if len(strings.TrimSpace(stripTags(text))) < 80 {
		return "", title, fmt.Errorf("extract too short")
	}
	return text, title, nil
}

var (
	reTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reScript  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNoscript = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reArticle = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	reMain    = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
	reP       = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
)

func extractTitle(html string) string {
	m := reTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(m[1]))
}

func extractMain(html string) string {
	html = reScript.ReplaceAllString(html, "")
	html = reStyle.ReplaceAllString(html, "")
	html = reNoscript.ReplaceAllString(html, "")
	if m := reArticle.FindStringSubmatch(html); len(m) > 1 {
		return cleanHTMLFragment(m[1])
	}
	if m := reMain.FindStringSubmatch(html); len(m) > 1 {
		return cleanHTMLFragment(m[1])
	}
	parts := reP.FindAllStringSubmatch(html, 40)
	var b strings.Builder
	b.WriteString("<div>")
	for _, p := range parts {
		if len(p) > 1 {
			t := strings.TrimSpace(stripTags(p[1]))
			if len(t) > 40 {
				b.WriteString("<p>")
				b.WriteString(escapeText(t))
				b.WriteString("</p>")
			}
		}
	}
	b.WriteString("</div>")
	return b.String()
}

func cleanHTMLFragment(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	// keep basic tags only by stripping scripts already; return as-is for sanitizer later
	return strings.TrimSpace(s)
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

func escapeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			if unicode.IsPrint(r) || unicode.IsSpace(r) {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
