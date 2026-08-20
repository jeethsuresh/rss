package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type Fetcher struct {
	Client *http.Client
	Parser *gofeed.Parser
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		Client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		Parser: gofeed.NewParser(),
	}
}

type FetchResult struct {
	StatusCode   int
	NotModified  bool
	ETag         string
	LastModified string
	Feed         *gofeed.Feed
	Body         []byte
}

func (f *Fetcher) Fetch(ctx context.Context, feedURL, etag, lastModified string) (*FetchResult, error) {
	if _, err := url.ParseRequestURI(feedURL); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidURL, err)
	}
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	res, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	defer res.Body.Close()
	out := &FetchResult{
		StatusCode:   res.StatusCode,
		ETag:         res.Header.Get("ETag"),
		LastModified: res.Header.Get("Last-Modified"),
	}
	if res.StatusCode == http.StatusNotModified {
		out.NotModified = true
		return out, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return out, fmt.Errorf("%w: status %d", domain.ErrNetwork, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	out.Body = body
	parsed, err := f.Parser.ParseString(string(body))
	if err != nil {
		return out, fmt.Errorf("%w: %v", domain.ErrParse, err)
	}
	if parsed == nil || (len(parsed.Items) == 0 && parsed.Title == "") {
		return out, domain.ErrInvalidFeed
	}
	out.Feed = parsed
	return out, nil
}

func (f *Fetcher) Discover(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", domain.ErrInvalidURL
	}
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop)")
	res, err := f.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	html := string(body)
	lower := strings.ToLower(html)
	for _, typ := range []string{`type="application/rss+xml"`, `type="application/atom+xml"`, `type='application/rss+xml'`, `type='application/atom+xml'`} {
		idx := strings.Index(lower, typ)
		if idx < 0 {
			continue
		}
		chunkStart := strings.LastIndex(lower[:idx], "<link")
		if chunkStart < 0 {
			continue
		}
		chunkEnd := strings.Index(lower[idx:], ">")
		if chunkEnd < 0 {
			continue
		}
		chunk := html[chunkStart : idx+chunkEnd]
		href := extractAttr(chunk, "href")
		if href == "" {
			continue
		}
		abs, err := resolveURL(pageURL, href)
		if err == nil {
			return abs, nil
		}
	}
	return "", domain.ErrInvalidFeed
}

func extractAttr(tag, name string) string {
	lower := strings.ToLower(tag)
	key := name + "="
	i := strings.Index(lower, key)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(key):]
	if len(rest) == 0 {
		return ""
	}
	quote := rest[0]
	if quote == '"' || quote == '\'' {
		end := strings.IndexByte(rest[1:], quote)
		if end < 0 {
			return ""
		}
		return rest[1 : 1+end]
	}
	end := strings.IndexAny(rest, " \t>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func resolveURL(base, ref string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}

func NormalizeArticles(feedID string, gf *gofeed.Feed, now time.Time) []domain.Article {
	out := make([]domain.Article, 0, len(gf.Items))
	for _, item := range gf.Items {
		if item == nil {
			continue
		}
		extID := strings.TrimSpace(item.GUID)
		itemURL := strings.TrimSpace(item.Link)
		title := strings.TrimSpace(item.Title)
		var published *time.Time
		if item.PublishedParsed != nil {
			published = item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = item.UpdatedParsed
		}
		var updated *time.Time
		if item.UpdatedParsed != nil {
			updated = item.UpdatedParsed
		}
		content := item.Content
		if content == "" {
			content = item.Description
		}
		summary := item.Description
		if len(summary) > 500 {
			summary = summary[:500]
		}
		author := ""
		if item.Author != nil {
			author = item.Author.Name
		}
		fp := Fingerprint(extID, itemURL, title, published)
		out = append(out, domain.Article{
			FeedID:       feedID,
			Title:        title,
			URL:          itemURL,
			Author:       author,
			Content:      content,
			Summary:      summary,
			PublishedAt:  published,
			UpdatedAt:    updated,
			ExternalID:   extID,
			DiscoveredAt: now,
			// Fingerprint encoded into storage via feeds package helper
		})
		_ = fp
	}
	return out
}

func Fingerprint(guid, itemURL, title string, published *time.Time) string {
	if guid != "" {
		return "guid:" + guid
	}
	if itemURL != "" {
		return "url:" + itemURL
	}
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte("|"))
	if published != nil {
		h.Write([]byte(published.UTC().Format(time.RFC3339)))
	}
	return "fp:" + hex.EncodeToString(h.Sum(nil))
}

func PreviewFromFeed(feedURL string, gf *gofeed.Feed) domain.FeedPreview {
	site := ""
	if gf.Link != "" {
		site = gf.Link
	}
	desc := gf.Description
	return domain.FeedPreview{
		URL:          feedURL,
		Title:        gf.Title,
		Description:  desc,
		SiteURL:      site,
		ArticleCount: len(gf.Items),
	}
}
