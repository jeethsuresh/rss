package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const ProtocolVersion = 1

type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type Response struct {
	ID     string `json:"id,omitempty"`
	Result any    `json:"result,omitempty"`
	Error  *Error `json:"error,omitempty"`
	Event  string `json:"event,omitempty"`
	Payload any   `json:"payload,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	svc  *application.Service
	log  *slog.Logger
	out  io.Writer
	mu   sync.Mutex
	done chan struct{}
}

func NewServer(svc *application.Service, log *slog.Logger, out io.Writer) *Server {
	s := &Server{svc: svc, log: log, out: out, done: make(chan struct{})}
	svc.Emit = s.Emit
	return s
}

func (s *Server) Emit(name string, payload any) {
	s.write(Response{Event: name, Payload: payload})
}

func (s *Server) write(resp Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func (s *Server) Serve(ctx context.Context, in io.Reader) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.write(Response{Error: &Error{Code: "INVALID_PARAMS", Message: "invalid json"}})
			continue
		}
		if req.Method == "system.shutdown" {
			s.write(Response{ID: req.ID, Result: map[string]any{"ok": true}})
			close(s.done)
			return nil
		}
		res, err := s.dispatch(ctx, req)
		if err != nil {
			s.write(Response{ID: req.ID, Error: mapError(err)})
			continue
		}
		s.write(Response{ID: req.ID, Result: res})
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *Server) Done() <-chan struct{} { return s.done }

func (s *Server) dispatch(ctx context.Context, req Request) (any, error) {
	switch req.Method {
	case "system.ping":
		return map[string]any{"ok": true, "version": s.svc.Version}, nil
	case "system.handshake":
		return map[string]any{"protocolVersion": ProtocolVersion, "version": s.svc.Version}, nil
	case "system.info":
		return map[string]any{"version": s.svc.Version, "dbPath": s.svc.DBPath, "protocolVersion": ProtocolVersion}, nil
	case "feeds.list":
		return s.svc.ListFeeds(ctx)
	case "feeds.get":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.GetFeed(ctx, p.ID)
	case "feeds.preview":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.PreviewFeed(ctx, p.URL)
	case "feeds.add":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.AddFeed(ctx, p.URL)
	case "feeds.remove":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, s.svc.RemoveFeed(ctx, p.ID)
	case "feeds.refresh":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.RefreshFeed(ctx, p.ID)
	case "feeds.refreshAll":
		return nil, s.svc.RefreshAll(ctx)
	case "feeds.setEnabled":
		var p struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.SetFeedEnabled(ctx, p.ID, p.Enabled)
	case "feeds.setPollInterval":
		var p struct {
			ID      string `json:"id"`
			Seconds int    `json:"seconds"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.SetFeedPollInterval(ctx, p.ID, p.Seconds)
	case "articles.list":
		var p domain.ArticleQuery
		_ = json.Unmarshal(req.Params, &p)
		// map JSON camelCase manually
		var raw map[string]any
		_ = json.Unmarshal(req.Params, &raw)
		if v, ok := raw["feedId"].(string); ok {
			p.FeedID = v
		}
		if v, ok := raw["folderId"].(string); ok {
			p.FolderID = v
		}
		if v, ok := raw["unreadOnly"].(bool); ok {
			p.UnreadOnly = v
		}
		if v, ok := raw["starredOnly"].(bool); ok {
			p.StarredOnly = v
		}
		if v, ok := raw["search"].(string); ok {
			p.Search = v
		}
		if v, ok := raw["limit"].(float64); ok {
			p.Limit = int(v)
		}
		if v, ok := raw["cursor"].(string); ok {
			p.Cursor = v
		}
		return s.svc.ListArticles(ctx, p)
	case "articles.get":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.GetArticle(ctx, p.ID)
	case "articles.markRead":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.MarkRead(ctx, p.ID, true)
	case "articles.markUnread":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.MarkRead(ctx, p.ID, false)
	case "articles.toggleStar":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.ToggleStar(ctx, p.ID)
	case "folders.list":
		return s.svc.ListFolders(ctx)
	case "folders.create":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.CreateFolder(ctx, p.Name)
	case "folders.remove":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, s.svc.RemoveFolder(ctx, p.ID)
	case "folders.assignFeed":
		var p struct {
			FolderID string `json:"folderId"`
			FeedID   string `json:"feedId"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.FolderID == "" || p.FeedID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, s.svc.AssignFeed(ctx, p.FolderID, p.FeedID)
	case "folders.unassignFeed":
		var p struct {
			FolderID string `json:"folderId"`
			FeedID   string `json:"feedId"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.FolderID == "" || p.FeedID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, s.svc.UnassignFeed(ctx, p.FolderID, p.FeedID)
	case "settings.get":
		return s.svc.GetSettings(ctx)
	case "settings.update":
		var raw map[string]any
		if err := json.Unmarshal(req.Params, &raw); err != nil {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.UpdateSettings(ctx, raw)
	case "feeds.exportUrls":
		text, err := s.svc.ExportFeedURLs(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"text": text}, nil
	case "feeds.importUrls":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.ImportFeedURLs(ctx, p.Text)
	case "stories.list":
		return s.svc.ListStories(ctx)
	case "stories.get":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.GetStory(ctx, p.ID)
	case "stories.markRead":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.MarkStoryRead(ctx, p.ID, true)
	case "stories.markUnread":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.MarkStoryRead(ctx, p.ID, false)
	case "stories.toggleStar":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.ToggleStoryStar(ctx, p.ID)
	case "ai.test":
		if s.svc.AI == nil {
			return nil, fmt.Errorf("ai unavailable")
		}
		return s.svc.AI.Test(ctx)
	case "ai.scan":
		var p struct {
			Window string `json:"window"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Window == "" {
			return nil, domain.ErrInvalidParams
		}
		if s.svc.AI == nil {
			return nil, fmt.Errorf("ai unavailable")
		}
		if err := s.svc.AI.ScanWindow(ctx, p.Window); err != nil {
			return nil, err
		}
		return map[string]any{"queued": true, "status": s.svc.AI.Status(ctx)}, nil
	case "ai.status":
		if s.svc.AI == nil {
			return domain.AIStatus{}, nil
		}
		return s.svc.AI.Status(ctx), nil
	case "ai.logs":
		var p struct {
			Limit int `json:"limit"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if s.svc.AI == nil {
			return []domain.AILogEntry{}, nil
		}
		return s.svc.AI.ListLogs(ctx, p.Limit)
	case "ai.retryFailed":
		if s.svc.AI == nil {
			return nil, fmt.Errorf("ai unavailable")
		}
		n, err := s.svc.AI.RetryFailed(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"requeued": n, "status": s.svc.AI.Status(ctx)}, nil
	case "readLater.add":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.URL == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.AddReadLater(ctx, p.URL)
	case "readLater.list":
		return s.svc.ListReadLater(ctx)
	case "articles.recrawl":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.RecrawlArticle(ctx, p.ID)
	case "articles.fetchLive":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return s.svc.FetchLiveArticle(ctx, p.ID)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupported, req.Method)
	}
}

var errUnsupported = errors.New("unsupported method")

func mapError(err error) *Error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return &Error{Code: "NOT_FOUND", Message: err.Error()}
	case errors.Is(err, domain.ErrInvalidURL):
		return &Error{Code: "INVALID_URL", Message: err.Error()}
	case errors.Is(err, domain.ErrInvalidFeed):
		return &Error{Code: "INVALID_FEED", Message: err.Error()}
	case errors.Is(err, domain.ErrInvalidParams):
		return &Error{Code: "INVALID_PARAMS", Message: err.Error()}
	case errors.Is(err, domain.ErrNetwork):
		return &Error{Code: "NETWORK_ERROR", Message: err.Error()}
	case errors.Is(err, domain.ErrParse):
		return &Error{Code: "PARSE_ERROR", Message: err.Error()}
	case errors.Is(err, errUnsupported):
		return &Error{Code: "UNSUPPORTED_METHOD", Message: err.Error()}
	default:
		return &Error{Code: "INTERNAL", Message: err.Error()}
	}
}
