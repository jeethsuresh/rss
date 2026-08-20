package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type Service struct {
	Articles domain.ArticleRepository
	Stories  domain.StoryRepository
	Settings domain.SettingsRepository
	Feeds    domain.FeedRepository
	Queue    domain.AIQueueRepository
	Logs     domain.AILogRepository
	Log      *slog.Logger
	Emit     func(name string, payload any)

	mu      sync.Mutex
	running bool
	client  *http.Client
}

func New(
	articles domain.ArticleRepository,
	stories domain.StoryRepository,
	settings domain.SettingsRepository,
	feeds domain.FeedRepository,
	queue domain.AIQueueRepository,
	logs domain.AILogRepository,
	log *slog.Logger,
) *Service {
	return &Service{
		Articles: articles,
		Stories:  stories,
		Settings: settings,
		Feeds:    feeds,
		Queue:    queue,
		Logs:     logs,
		Log:      log,
		client:   &http.Client{Timeout: 15 * time.Minute},
	}
}

func (s *Service) appendLog(ctx context.Context, level, articleID, message, detail string) {
	entry := domain.AILogEntry{
		ID:        uuid.NewString(),
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		ArticleID: articleID,
		Message:   message,
		Detail:    detail,
	}
	_ = s.Logs.Append(ctx, entry)
	if s.Emit != nil {
		s.Emit("ai.log", entry)
	}
}

func (s *Service) Status(ctx context.Context) domain.AIStatus {
	pending, running, done, failed, _ := s.Queue.Counts(ctx)
	s.mu.Lock()
	isRunning := s.running
	s.mu.Unlock()
	return domain.AIStatus{
		Running:   isRunning || pending > 0 || running > 0,
		Processed: done,
		Total:     pending + running + done + failed,
		Pending:   pending,
		Failed:    failed,
	}
}

func (s *Service) emitStatus(ctx context.Context) {
	if s.Emit != nil {
		s.Emit("ai.status", s.Status(ctx))
	}
}

func (s *Service) Resume(ctx context.Context) {
	_ = s.Queue.ResetRunning(ctx)
	s.appendLog(ctx, "info", "", "AI queue resumed", "")
	s.Kick(ctx)
}

func (s *Service) Kick(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.worker()
}

func (s *Service) Enqueue(ids ...string) {
	ctx := context.Background()
	for _, id := range ids {
		if id == "" {
			continue
		}
		if err := s.Queue.Enqueue(ctx, id); err != nil {
			s.Log.Warn("enqueue", "id", id, "err", err)
			continue
		}
		s.appendLog(ctx, "info", id, "queued for AI triage", "")
	}
	s.emitStatus(ctx)
	s.Kick(ctx)
}

func (s *Service) ScanWindow(ctx context.Context, window string) error {
	settings, err := s.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if !settings.AIEnabled || strings.TrimSpace(settings.AIBaseURL) == "" {
		return fmt.Errorf("%w: AI disabled or missing base URL", domain.ErrInvalidParams)
	}
	var since time.Time
	switch window {
	case "24h":
		since = time.Now().UTC().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	default:
		return domain.ErrInvalidParams
	}
	ids, err := s.Articles.ListIDsSince(ctx, since)
	if err != nil {
		return err
	}
	s.appendLog(ctx, "info", "", fmt.Sprintf("scan window %s (%d articles)", window, len(ids)), "")
	s.Enqueue(ids...)
	return nil
}

func (s *Service) RetryFailed(ctx context.Context) (int, error) {
	n, err := s.Queue.RetryFailed(ctx)
	if err != nil {
		return 0, err
	}
	s.appendLog(ctx, "info", "", fmt.Sprintf("retry failed: %d re-queued", n), "")
	s.emitStatus(ctx)
	s.Kick(ctx)
	return n, nil
}

func (s *Service) ListLogs(ctx context.Context, limit int) ([]domain.AILogEntry, error) {
	return s.Logs.List(ctx, limit)
}

func (s *Service) Test(ctx context.Context) (*domain.AITestResult, error) {
	settings, err := s.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(settings.AIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return &domain.AITestResult{OK: false, Message: err.Error()}, nil
	}
	res, err := s.client.Do(req)
	if err != nil {
		return &domain.AITestResult{OK: false, Message: err.Error()}, nil
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return &domain.AITestResult{OK: false, Message: fmt.Sprintf("status %d: %s", res.StatusCode, string(body))}, nil
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &parsed)
	models := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		models = append(models, m.ID)
	}
	return &domain.AITestResult{OK: true, Message: "connected", Models: models}, nil
}

func (s *Service) worker() {
	ctx := context.Background()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.emitStatus(ctx)
	}()

	for {
		id, ok, err := s.Queue.ClaimNext(ctx)
		if err != nil {
			s.appendLog(ctx, "error", "", "claim next failed", err.Error())
			return
		}
		if !ok {
			return
		}
		s.emitStatus(ctx)
		s.appendLog(ctx, "info", id, "processing", "")
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		err = s.processArticle(jobCtx, id)
		cancel()
		if err != nil {
			_ = s.Queue.MarkFailed(ctx, id, err.Error())
			s.appendLog(ctx, "error", id, "failed", err.Error())
		} else {
			_ = s.Queue.MarkDone(ctx, id)
			s.appendLog(ctx, "info", id, "done", "")
		}
		s.emitStatus(ctx)
	}
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []any         `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Service) processArticle(ctx context.Context, id string) error {
	settings, err := s.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if !settings.AIEnabled {
		return fmt.Errorf("AI disabled")
	}
	article, err := s.Articles.Get(ctx, id)
	if err != nil {
		return err
	}

	bodyForAI := triageBody(article)

	system := `You triage RSS/read-later articles for a local reader.
You may call tools. When finished, output a single JSON object (no markdown):
{"priority":"high|medium|low","storyAction":"none|create|join","storyId":"","storyTitle":"","storySummary":"","memberIds":["..."]}
Rules:
- Prefer crawled/full-page text when available and not marked unreliable.
- If crawled text looks wrong (nav chrome, paywall, empty), call mark_crawl_unreliable then use the feed/RSS preview text.
- Use search_articles to find related coverage before create/join.
- storyAction=join needs storyId or memberIds from search; create needs title/summary/memberIds including this article.`

	user := fmt.Sprintf("Article id=%s feed=%q title=%q crawlStatus=%s unreliable=%v\nText:\n%s",
		article.ID, article.FeedTitle, article.Title, article.CrawlStatus, article.CrawlUnreliable, truncate(bodyForAI, 3500))

	messages := []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search_articles",
				"description": "Search existing articles for similar coverage",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
						"limit": map[string]any{"type": "integer"},
					},
					"required": []string{"query"},
				},
			},
		},
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "mark_crawl_unreliable",
				"description": "Mark this article's crawled page as bad; continue using RSS/feed preview. Also increments per-feed bad crawl stats.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"reason": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	model := settings.AIModel
	var final string
	for round := 0; round < 5; round++ {
		msg, err := s.chat(ctx, settings.AIBaseURL, model, messages, tools)
		if err != nil {
			return err
		}
		if len(msg.ToolCalls) > 0 {
			messages = append(messages, msg)
			for _, tc := range msg.ToolCalls {
				result, toolErr := s.runTool(ctx, article, tc)
				content := result
				if toolErr != nil {
					content = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
				}
				messages = append(messages, chatMessage{
					Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: content,
				})
			}
			// refresh article after mark_crawl_unreliable
			if refreshed, err := s.Articles.Get(ctx, id); err == nil {
				article = refreshed
			}
			continue
		}
		final = strings.TrimSpace(msg.Content)
		break
	}
	if final == "" {
		return fmt.Errorf("empty AI response")
	}
	final = extractJSON(final)

	var out struct {
		Priority     string   `json:"priority"`
		StoryAction  string   `json:"storyAction"`
		StoryID      string   `json:"storyId"`
		StoryTitle   string   `json:"storyTitle"`
		StorySummary string   `json:"storySummary"`
		MemberIDs    []string `json:"memberIds"`
	}
	if err := json.Unmarshal([]byte(final), &out); err != nil {
		return fmt.Errorf("parse AI json: %w (%s)", err, truncate(final, 200))
	}

	pri := domain.Priority(strings.ToLower(out.Priority))
	switch pri {
	case domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow:
	default:
		pri = domain.PriorityMedium
	}
	if err := s.Articles.SetPriority(ctx, article.ID, pri); err != nil {
		return err
	}

	switch out.StoryAction {
	case "join":
		storyID := out.StoryID
		if storyID == "" && len(out.MemberIDs) > 0 {
			if st, err := s.Stories.FindStoryForArticle(ctx, out.MemberIDs[0]); err == nil {
				storyID = st.ID
			}
		}
		if storyID != "" {
			_ = s.Stories.AddMember(ctx, storyID, article.ID)
		}
	case "create":
		members := uniqueStrings(append(out.MemberIDs, article.ID))
		now := time.Now().UTC()
		st := &domain.Story{
			ID:        uuid.NewString(),
			Title:     firstNonEmpty(out.StoryTitle, article.Title),
			Summary:   firstNonEmpty(out.StorySummary, truncate(stripTags(article.Summary), 280)),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.Stories.Create(ctx, st); err != nil {
			return err
		}
		if err := s.Stories.SetMembers(ctx, st.ID, members); err != nil {
			return err
		}
		if s.Emit != nil {
			s.Emit("story.updated", map[string]any{"storyId": st.ID})
		}
	}

	if s.Emit != nil {
		s.Emit("article.updated", map[string]any{"articleId": article.ID, "priority": pri})
	}
	return nil
}

func triageBody(a *domain.Article) string {
	if !a.CrawlUnreliable && a.CrawlStatus == domain.CrawlOK && strings.TrimSpace(a.CrawledContent) != "" {
		return stripTags(a.CrawledContent)
	}
	if strings.TrimSpace(a.RSSContent) != "" {
		return stripTags(a.RSSContent)
	}
	if strings.TrimSpace(a.Content) != "" {
		return stripTags(a.Content)
	}
	return stripTags(a.Summary)
}

func (s *Service) runTool(ctx context.Context, article *domain.Article, tc toolCall) (string, error) {
	switch tc.Function.Name {
	case "search_articles":
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		arts, err := s.Articles.SearchCompact(ctx, args.Query, args.Limit)
		if err != nil {
			return "", err
		}
		type hit struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			FeedTitle string `json:"feedTitle"`
			Summary   string `json:"summary"`
			StoryID   string `json:"storyId,omitempty"`
		}
		hits := make([]hit, 0, len(arts))
		for _, a := range arts {
			hits = append(hits, hit{
				ID: a.ID, Title: a.Title, FeedTitle: a.FeedTitle,
				Summary: truncate(stripTags(a.Summary), 180), StoryID: a.StoryID,
			})
		}
		b, _ := json.Marshal(hits)
		return string(b), nil
	case "mark_crawl_unreliable":
		var args struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		reason := firstNonEmpty(args.Reason, "model flagged crawl as unreliable")
		_ = s.Articles.SetCrawlResult(ctx, article.ID, domain.CrawlFailed, article.CrawledContent, reason, true)
		_ = s.Feeds.RecordCrawlResult(ctx, article.FeedID, true)
		s.appendLog(ctx, "warn", article.ID, "mark_crawl_unreliable", reason)
		if s.Emit != nil {
			s.Emit("article.updated", map[string]any{"articleId": article.ID, "crawlUnreliable": true})
		}
		return `{"ok":true,"use":"rss_preview"}`, nil
	default:
		return "", fmt.Errorf("unknown tool %s", tc.Function.Name)
	}
}

func (s *Service) chat(ctx context.Context, baseURL, model string, messages []chatMessage, tools []any) (chatMessage, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if model == "" {
		model = "local-model"
	}
	payload := chatRequest{Model: model, Messages: messages, Tools: tools}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return chatMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return chatMessage{}, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return chatMessage{}, fmt.Errorf("chat status %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return chatMessage{}, err
	}
	if parsed.Error != nil {
		return chatMessage{}, fmt.Errorf("%s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("no choices")
	}
	return parsed.Choices[0].Message, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
