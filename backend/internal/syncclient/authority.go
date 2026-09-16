package syncclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const maxEventStreamMessageBytes = 16 * 1024 * 1024

var authorityMutationMethods = map[string]bool{
	"feeds.add": true, "feeds.remove": true, "feeds.refresh": true,
	"feeds.refreshAll": true, "feeds.setEnabled": true, "feeds.setPollInterval": true,
	"feeds.importUrls":  true,
	"articles.markRead": true, "articles.markUnread": true, "articles.markAllRead": true,
	"articles.toggleStar": true, "articles.recrawl": true, "articles.fetchLive": true,
	"articles.setExtract": true,
	"readLater.add":       true, "readLater.addFromArticle": true, "readLater.archive": true,
	"readLater.unarchive": true, "readLater.remove": true,
	"sports.followed.set": true, "sports.followed.toggle": true,
	"sports.game.watch": true, "sports.game.unwatch": true,
	"sports.f1.race.watch": true, "sports.f1.race.unwatch": true,
	"stories.markRead": true, "stories.markUnread": true, "stories.toggleStar": true,
	"stories.voteArticle": true, "stories.voteStory": true, "stories.reindex": true,
	"stories.split":  true,
	"folders.create": true, "folders.remove": true, "folders.assignFeed": true,
	"folders.unassignFeed": true,
	"settings.update":      true,
	"ai.scan":              true, "ai.retryFailed": true,
}

func IsAuthorityMethod(method string) bool {
	for _, prefix := range []string{
		"feeds.", "articles.", "readLater.", "sports.", "stories.",
		"folders.", "settings.", "ai.",
	} {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return method == "errors.list"
}

func IsAuthorityMutation(method string) bool { return authorityMutationMethods[method] }

func (c *Client) RPC(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	serverURL := strings.TrimRight(strings.TrimSpace(c.Config.ServerURL), "/")
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}
	token, err := c.login(ctx, serverURL)
	if err != nil {
		return nil, err
	}
	request := struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}{Method: method, Params: params}
	var result json.RawMessage
	status, err := c.doJSON(ctx, http.MethodPost, serverURL+"/v1/rpc", token, request, &result)
	var statusErr *remoteStatusError
	if errors.As(err, &statusErr) && statusErr.Status == http.StatusUnauthorized {
		c.clearToken()
		token, err = c.login(ctx, serverURL)
		if err == nil {
			status, err = c.doJSON(ctx, http.MethodPost, serverURL+"/v1/rpc", token, request, &result)
		}
	}
	if err != nil {
		return nil, err
	}
	if status >= http.StatusMultipleChoices {
		return nil, &remoteStatusError{Status: status}
	}
	return result, nil
}

func (c *Client) StreamEvents(ctx context.Context, emit func(string, any)) error {
	serverURL := strings.TrimRight(strings.TrimSpace(c.Config.ServerURL), "/")
	token, err := c.login(ctx, serverURL)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/v1/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// SSE is intentionally long-lived. Keep the ordinary RPC timeout on
	// c.HTTP, but do not let it terminate a healthy event stream every 90s.
	streamHTTP := *c.HTTP
	streamHTTP.Timeout = 0
	res, err := streamHTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		c.clearToken()
	}
	if res.StatusCode >= http.StatusMultipleChoices {
		return &remoteStatusError{Status: res.StatusCode}
	}
	scanner := bufio.NewScanner(res.Body)
	// Sports and extracted-content invalidations can legitimately carry more
	// than Scanner's 64 KiB default token limit. Match the desktop IPC ceiling
	// while retaining an upper bound for malformed or hostile streams.
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventStreamMessageBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Event   string `json:"event"`
			Payload any    `json:"payload"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) == nil && event.Event != "" {
			emit(event.Event, event.Payload)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ctx.Err()
}

func (m *Manager) RPC(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	m.mu.Lock()
	client := m.client
	serverURL := m.serverURL
	username := m.username
	m.mu.Unlock()
	if client == nil {
		return nil, ErrNotConnected
	}

	mutationID := ""
	if IsAuthorityMutation(method) {
		mutationID = uuid.NewString()
		payload := string(params)
		if strings.TrimSpace(payload) == "" {
			payload = "{}"
		}
		if _, err := m.DB.SQL.ExecContext(ctx, `
			INSERT INTO local_authority_mutations(
			  id, server_url, username, method, params, status, created_at
			) VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
			mutationID, serverURL, username, method, payload, nowText()); err != nil {
			return nil, err
		}
	}

	result, err := client.RPC(ctx, method, params)
	if err != nil {
		if mutationID != "" {
			_, _ = m.DB.SQL.ExecContext(ctx, `
				UPDATE local_authority_mutations
				SET status='rolled_back', error=?, completed_at=? WHERE id=?`,
				err.Error(), nowText(), mutationID)
		}
		return nil, err
	}
	if mutationID != "" {
		if _, err := m.DB.SQL.ExecContext(ctx, `
			UPDATE local_authority_mutations
			SET status='committed', error='', completed_at=? WHERE id=?`, nowText(), mutationID); err != nil {
			return nil, err
		}
		// The server has committed. Pull its CRDT records immediately so the
		// offline SQLite replica reflects the accepted mutation as well.
		if _, syncErr := m.syncClient(ctx, client); syncErr != nil && m.Log != nil {
			m.Log.Warn("materialize server mutation locally", "method", method, "err", syncErr)
		}
	}
	return result, nil
}
