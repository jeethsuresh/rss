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
	Log      *slog.Logger
	Emit     func(name string, payload any)

	mu        sync.Mutex
	queue     []string
	queued    map[string]bool
	running   bool
	processed int
	total     int
	lastError string
	client    *http.Client
}

func New(articles domain.ArticleRepository, stories domain.StoryRepository, settings domain.SettingsRepository, log *slog.Logger) *Service {
	return &Service{
		Articles: articles,
		Stories:  stories,
		Settings: settings,
		Log:      log,
		queued:   map[string]bool{},
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (s *Service) Status() domain.AIStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.AIStatus{
		Running:   s.running || len(s.queue) > 0,
		Processed: s.processed,
		Total:     s.total,
		LastError: s.lastError,
	}
}

func (s *Service) emitStatus() {
	if s.Emit != nil {
		s.Emit("ai.status", s.Status())
	}
}

func (s *Service) Enqueue(ids ...string) {
	s.mu.Lock()
	for _, id := range ids {
		if id == "" || s.queued[id] {
			continue
		}
		s.queued[id] = true
		s.queue = append(s.queue, id)
		s.total++
	}
	start := !s.running && len(s.queue) > 0
	s.mu.Unlock()
	s.emitStatus()
	if start {
		go s.worker()
	}
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
	s.Enqueue(ids...)
	return nil
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
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	s.emitStatus()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.emitStatus()
	}()

	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			s.mu.Unlock()
			return
		}
		id := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		err := s.processArticle(ctx, id)
		cancel()

		s.mu.Lock()
		delete(s.queued, id)
		s.processed++
		if err != nil {
			s.lastError = err.Error()
			s.Log.Warn("ai process article", "id", id, "err", err)
		}
		s.mu.Unlock()
		s.emitStatus()
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
		return nil
	}
	article, err := s.Articles.Get(ctx, id)
	if err != nil {
		return err
	}

	system := `You triage RSS articles for a local reader.
Respond by calling tools when helpful, then finish with a single JSON object (no markdown) of the form:
{"priority":"high|medium|low","storyAction":"none|create|join","storyId":"","storyTitle":"","storySummary":"","memberIds":["..."]}
Rules:
- priority reflects importance/urgency for the user.
- Use search_articles to find related coverage before creating/joining a story.
- storyAction=join requires storyId of an existing story OR memberIds including known article ids from search.
- storyAction=create needs storyTitle, storySummary, and memberIds including this article id.
- Prefer none when no strong similarity.`

	user := fmt.Sprintf("Article id=%s feed=%q title=%q summary=%q",
		article.ID, article.FeedTitle, article.Title, truncate(stripTags(article.Summary), 600))

	messages := []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "search_articles",
				"description": "Search existing articles by keywords for similar coverage",
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
	}

	model := settings.AIModel
	var final string
	for round := 0; round < 4; round++ {
		msg, err := s.chat(ctx, settings.AIBaseURL, model, messages, tools)
		if err != nil {
			return err
		}
		if len(msg.ToolCalls) > 0 {
			messages = append(messages, msg)
			for _, tc := range msg.ToolCalls {
				result, toolErr := s.runTool(ctx, tc)
				content := result
				if toolErr != nil {
					content = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
				}
				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    content,
				})
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
		if len(members) < 2 {
			// still allow single-member story seed for later joins
		}
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

func (s *Service) runTool(ctx context.Context, tc toolCall) (string, error) {
	name := tc.Function.Name
	switch name {
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
	default:
		return "", fmt.Errorf("unknown tool %s", name)
	}
}

func (s *Service) chat(ctx context.Context, baseURL, model string, messages []chatMessage, tools []any) (chatMessage, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if model == "" {
		// LM Studio often accepts any placeholder; try empty then "local-model"
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
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
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
