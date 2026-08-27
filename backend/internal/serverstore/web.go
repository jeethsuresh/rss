package serverstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type webCursor struct {
	Timestamp string `json:"timestamp"`
	ID        string `json:"id"`
}

func (s *Store) ListWebFeeds(ctx context.Context, userID string) ([]domain.Feed, error) {
	feeds, err := s.ListFeeds(ctx, userID)
	if err != nil {
		return nil, err
	}
	var unread int
	if err := s.db.SQL.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM user_read_later
		WHERE user_id=? AND archived_at IS NULL AND is_read=0`, userID).Scan(&unread); err != nil {
		return nil, err
	}
	feeds = append(feeds, domain.Feed{
		ID: "read-later", Title: "Read Later", Enabled: true,
		UnreadCount: unread, IsReadLater: true,
		CreatedAt: s.now(), UpdatedAt: s.now(),
	})
	return feeds, nil
}

func (s *Store) ListAILogs(ctx context.Context, userID string, limit int) ([]domain.AILogEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT DISTINCT logs.id, logs.ts, logs.level, logs.article_id, logs.message, logs.detail
		FROM ai_logs logs
		JOIN articles a ON a.id=logs.article_id
		JOIN user_feeds uf ON uf.feed_id=a.feed_id
		WHERE uf.user_id=? AND uf.present=1
		ORDER BY logs.ts DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []domain.AILogEntry{}
	for rows.Next() {
		var entry domain.AILogEntry
		if err := rows.Scan(&entry.ID, &entry.TS, &entry.Level, &entry.ArticleID, &entry.Message, &entry.Detail); err != nil {
			return nil, err
		}
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	return logs, nil
}

func encodeWebCursor(timestamp, id string) string {
	raw, _ := json.Marshal(webCursor{Timestamp: timestamp, ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeWebCursor(raw string) (webCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return webCursor{}, err
	}
	var cursor webCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.Timestamp == "" || cursor.ID == "" {
		return webCursor{}, domain.ErrInvalidParams
	}
	return cursor, nil
}

func (s *Store) SetFeedMembership(ctx context.Context, userID, rawURL string, present bool) (*domain.Feed, error) {
	feedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}
	clock := s.now().UnixMilli()
	var current int64
	err = s.db.SQL.QueryRowContext(ctx, `
		SELECT uf.logical_clock FROM user_feeds uf JOIN feeds f ON f.id=uf.feed_id
		WHERE uf.user_id=? AND f.url=?`, userID, feedURL).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if current >= clock {
		clock = current + 1
	}
	_, err = s.SyncFeeds(ctx, userID, SyncRequest{Ops: []FeedOp{{
		OpID: uuid.NewString(), FeedURL: feedURL, Present: present,
		LogicalClock: clock, DeviceID: "server-web", ClientCreatedAt: formatTime(s.now()),
	}}})
	if err != nil {
		return nil, err
	}
	var feedID string
	if err := s.db.SQL.QueryRowContext(ctx, `SELECT id FROM feeds WHERE url=?`, feedURL).Scan(&feedID); err != nil {
		return nil, err
	}
	if present {
		_, _ = s.db.SQL.ExecContext(ctx, `
			UPDATE user_feeds SET poll_interval_seconds=COALESCE((
			  SELECT default_poll_interval_seconds FROM user_settings WHERE user_id=?
			), 3600), enabled=1 WHERE user_id=? AND feed_id=?`, userID, userID, feedID)
		return s.GetUserFeed(ctx, userID, feedID)
	}
	return nil, nil
}

func (s *Store) RemoveFeedMembership(ctx context.Context, userID, feedID string) error {
	var feedURL string
	if err := s.db.SQL.QueryRowContext(ctx, `
		SELECT f.url FROM feeds f JOIN user_feeds uf ON uf.feed_id=f.id
		WHERE uf.user_id=? AND uf.feed_id=? AND uf.present=1`, userID, feedID).Scan(&feedURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	_, err := s.SetFeedMembership(ctx, userID, feedURL, false)
	return err
}

func (s *Store) GetUserFeed(ctx context.Context, userID, feedID string) (*domain.Feed, error) {
	feeds, err := s.ListFeeds(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range feeds {
		if feeds[i].ID == feedID {
			return &feeds[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *Store) SetUserFeedEnabled(ctx context.Context, userID, feedID string, enabled bool) (*domain.Feed, error) {
	res, err := s.db.SQL.ExecContext(ctx, `
		UPDATE user_feeds SET enabled=?, updated_at=?
		WHERE user_id=? AND feed_id=? AND present=1`, boolInt(enabled), formatTime(s.now()), userID, feedID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	if enabled {
		_ = s.QueueFeedRefresh(ctx, userID, feedID)
	}
	return s.GetUserFeed(ctx, userID, feedID)
}

func (s *Store) SetUserFeedPollInterval(ctx context.Context, userID, feedID string, seconds int) (*domain.Feed, error) {
	if seconds < 60 {
		seconds = 60
	}
	if seconds > 86400 {
		seconds = 86400
	}
	res, err := s.db.SQL.ExecContext(ctx, `
		UPDATE user_feeds SET poll_interval_seconds=?, updated_at=?
		WHERE user_id=? AND feed_id=? AND present=1`, seconds, formatTime(s.now()), userID, feedID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return s.GetUserFeed(ctx, userID, feedID)
}

func (s *Store) QueueFeedRefresh(ctx context.Context, userID, feedID string) error {
	res, err := s.db.SQL.ExecContext(ctx, `
		UPDATE feed_fetch_state SET next_fetch_at=?, updated_at=?
		WHERE feed_id=? AND EXISTS (
		  SELECT 1 FROM user_feeds WHERE user_id=? AND feed_id=? AND present=1 AND enabled=1
		)`, formatTime(s.now()), formatTime(s.now()), feedID, userID, feedID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) QueueAllFeedRefreshes(ctx context.Context, userID string) error {
	_, err := s.db.SQL.ExecContext(ctx, `
		UPDATE feed_fetch_state SET next_fetch_at=?, updated_at=?
		WHERE feed_id IN (
		  SELECT feed_id FROM user_feeds WHERE user_id=? AND present=1 AND enabled=1
		)`, formatTime(s.now()), formatTime(s.now()), userID)
	return err
}

func (s *Store) ListArticlesPage(ctx context.Context, userID string, q domain.ArticleQuery) (domain.ArticleListResult, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return domain.ArticleListResult{}, err
	}
	oldest := settings.DefaultSort == "oldest"
	where := []string{"uf.user_id=?", "uf.present=1", "a.is_read_later=0"}
	args := []any{userID}
	if q.FeedID != "" {
		where = append(where, "a.feed_id=?")
		args = append(args, q.FeedID)
	}
	if q.FolderID != "" {
		where = append(where, `EXISTS (
		  SELECT 1 FROM user_feed_folders uff
		  WHERE uff.user_id=uf.user_id AND uff.folder_id=? AND uff.feed_id=a.feed_id
		)`)
		args = append(args, q.FolderID)
	}
	if q.UnreadOnly {
		where = append(where, "COALESCE(uas.is_read, 0)=0")
	}
	if q.StarredOnly {
		where = append(where, "COALESCE(uas.is_starred, 0)=1")
	}
	if search := strings.TrimSpace(q.Search); search != "" {
		where = append(where, "(a.title LIKE ? OR a.author LIKE ? OR a.summary LIKE ? OR a.content LIKE ?)")
		needle := "%" + search + "%"
		args = append(args, needle, needle, needle, needle)
	}
	if q.Cursor != "" {
		cursor, err := decodeWebCursor(q.Cursor)
		if err != nil {
			return domain.ArticleListResult{}, err
		}
		operator := "<"
		idOperator := "<"
		if oldest {
			operator, idOperator = ">", ">"
		}
		where = append(where, fmt.Sprintf(`(
		  COALESCE(a.published_at, a.discovered_at) %s ? OR
		  (COALESCE(a.published_at, a.discovered_at)=? AND a.id %s ?)
		)`, operator, idOperator))
		args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.ID)
	}
	order := "DESC"
	if oldest {
		order = "ASC"
	}
	args = append(args, limit+1)
	rows, err := s.db.SQL.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.id, COALESCE(uas.is_read, 0), COALESCE(uas.is_starred, 0),
		       COALESCE(a.published_at, a.discovered_at)
		FROM articles a
		JOIN user_feeds uf ON uf.feed_id=a.feed_id
		LEFT JOIN user_article_state uas ON uas.user_id=uf.user_id AND uas.article_id=a.id
		WHERE %s
		ORDER BY COALESCE(a.published_at, a.discovered_at) %s, a.id %s
		LIMIT ?`, strings.Join(where, " AND "), order, order), args...)
	if err != nil {
		return domain.ArticleListResult{}, err
	}
	defer rows.Close()
	type rowState struct {
		ID, Timestamp string
		Read, Starred int
	}
	states := []rowState{}
	for rows.Next() {
		var state rowState
		if err := rows.Scan(&state.ID, &state.Read, &state.Starred, &state.Timestamp); err != nil {
			return domain.ArticleListResult{}, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return domain.ArticleListResult{}, err
	}
	var next string
	if len(states) > limit {
		last := states[limit-1]
		next = encodeWebCursor(last.Timestamp, last.ID)
		states = states[:limit]
	}
	articles := make([]domain.Article, 0, len(states))
	for _, state := range states {
		article, err := s.articles.Get(ctx, state.ID)
		if err != nil {
			return domain.ArticleListResult{}, err
		}
		article.IsRead = state.Read == 1
		article.IsStarred = state.Starred == 1
		compactListArticle(article)
		articles = append(articles, *article)
	}
	return domain.ArticleListResult{Articles: articles, NextCursor: next}, nil
}

func (s *Store) MarkAllArticlesRead(ctx context.Context, userID string, q domain.ArticleQuery) (int, error) {
	q.Cursor = ""
	q.Limit = 200
	updated := 0
	for {
		page, err := s.ListArticlesPage(ctx, userID, q)
		if err != nil {
			return updated, err
		}
		if len(page.Articles) == 0 {
			return updated, nil
		}
		value := true
		for _, article := range page.Articles {
			if article.IsRead {
				continue
			}
			if _, err := s.SetArticleState(ctx, userID, article.ID, ArticleStatePatch{IsRead: &value}); err != nil {
				return updated, err
			}
			updated++
		}
		if page.NextCursor == "" {
			return updated, nil
		}
		q.Cursor = page.NextCursor
	}
}

func (s *Store) GetStory(ctx context.Context, userID, storyID string) (*domain.Story, error) {
	story, err := s.stories.Get(ctx, storyID)
	if err != nil {
		return nil, err
	}
	articles := make([]domain.Article, 0, len(story.Articles))
	ids := make([]string, 0, len(story.ArticleIDs))
	for _, article := range story.Articles {
		member, err := s.GetArticle(ctx, userID, article.ID)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		articles = append(articles, *member)
		ids = append(ids, member.ID)
	}
	if len(articles) == 0 {
		return nil, domain.ErrNotFound
	}
	story.Articles = articles
	story.ArticleIDs = ids
	story.MemberCount = len(articles)
	story.IsRead = true
	for _, article := range articles {
		if !article.IsRead {
			story.IsRead = false
			break
		}
	}
	var read, starred int
	err = s.db.SQL.QueryRowContext(ctx, `
		SELECT is_read, is_starred FROM user_story_state WHERE user_id=? AND story_id=?`,
		userID, storyID).Scan(&read, &starred)
	if err == nil {
		story.IsStarred = starred == 1
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return story, nil
}

func (s *Store) ReadLaterArticle(ctx context.Context, userID, entryID string) (*domain.Article, error) {
	item, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	return readLaterArticle(item), nil
}

func (s *Store) ListReadLaterArticles(ctx context.Context, userID, filter, search string) ([]domain.Article, error) {
	items, err := s.ListReadLater(ctx, userID, filter, search, 500)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Article, 0, len(items))
	for i := range items {
		article := readLaterArticle(&items[i])
		compactListArticle(article)
		out = append(out, *article)
	}
	return out, nil
}

// List responses stay compact so a page of crawled documents cannot exhaust
// the desktop RPC transport. Full content is available from articles.get (or
// articles.fetchLive for read-later entries) when the item becomes active.
func compactListArticle(article *domain.Article) {
	article.Content = ""
	article.RSSContent = ""
	article.CrawledContent = ""
	article.LiveContent = ""
	article.ReaderContent = ""
	const maxSummaryBytes = 4 << 10
	if len(article.Summary) > maxSummaryBytes {
		article.Summary = article.Summary[:maxSummaryBytes]
	}
}

func readLaterArticle(item *ReadLaterItem) *domain.Article {
	title := item.Title
	if strings.TrimSpace(title) == "" {
		title = item.URL
	}
	return &domain.Article{
		ID: item.ID, FeedID: "readlater-local", FeedTitle: "Read later",
		Title: title, URL: item.URL, IsRead: item.IsRead, IsStarred: item.IsStarred,
		IsReadLater: true, ArchivedAt: item.ArchivedAt, Priority: domain.PriorityNone,
		CrawledContent: item.CrawledContent, LiveContent: item.CrawledContent,
		ReaderContent: item.ReaderContent, CrawlStatus: domain.CrawlStatus(item.CrawlStatus),
		CrawlError: item.CrawlError, DiscoveredAt: item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	}
}

func (s *Store) SetReadLaterArticleState(ctx context.Context, userID, entryID string, read, starred *bool) (*domain.Article, error) {
	item, err := s.SetReadLaterState(ctx, userID, entryID, ReadLaterPatch{IsRead: read, IsStarred: starred})
	if err != nil {
		return nil, err
	}
	return readLaterArticle(item), nil
}

func (s *Store) SetReadLaterArchived(ctx context.Context, userID, entryID string, archived bool) (*domain.Article, error) {
	item, err := s.SetReadLaterState(ctx, userID, entryID, ReadLaterPatch{Archived: &archived})
	if err != nil {
		return nil, err
	}
	return readLaterArticle(item), nil
}

func (s *Store) QueueReadLaterRefresh(ctx context.Context, userID, entryID string) (*domain.Article, error) {
	item, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.SQL.ExecContext(ctx, `
		UPDATE web_documents SET crawl_status='pending', crawl_error='', next_fetch_at=?, updated_at=?
		WHERE id=?`, formatTime(s.now()), formatTime(s.now()), item.SharedDocumentID)
	if err != nil {
		return nil, err
	}
	return s.ReadLaterArticle(ctx, userID, entryID)
}

func (s *Store) QueueArticleRefresh(ctx context.Context, userID, articleID string) (*domain.Article, error) {
	article, err := s.GetArticle(ctx, userID, articleID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.SQL.ExecContext(ctx, `
		UPDATE articles SET crawl_status='pending', crawl_error='', crawl_retryable=1 WHERE id=?`, articleID)
	if err != nil {
		return nil, err
	}
	if normalized, err := NormalizeURL(article.URL); err == nil {
		_, _ = s.db.SQL.ExecContext(ctx, `
			UPDATE web_documents SET crawl_status='pending', crawl_error='', next_fetch_at=?, updated_at=?
			WHERE normalized_url=?`, formatTime(s.now()), formatTime(s.now()), normalized)
	}
	return s.GetArticle(ctx, userID, articleID)
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	_, err := s.db.SQL.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash=?`, hex.EncodeToString(hash[:]))
	return err
}
