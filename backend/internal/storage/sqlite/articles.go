package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type ArticleRepo struct{ db *DB }

func NewArticleRepo(db *DB) *ArticleRepo { return &ArticleRepo{db: db} }

const articleSelect = `
		SELECT a.id, a.feed_id, a.title, a.url, a.author, a.content, a.summary,
		       a.published_at, a.updated_at, a.external_id, a.is_read, a.is_starred,
		       a.priority, COALESCE(a.story_id, ''),
		       a.rss_content, a.crawled_content, a.live_content, a.crawl_status,
		       a.crawl_error, a.crawl_unreliable, a.is_read_later, a.archived_at,
		       a.discovered_at, f.title
		FROM articles a
		JOIN feeds f ON f.id = a.feed_id`

func (r *ArticleRepo) List(ctx context.Context, q domain.ArticleQuery) (domain.ArticleListResult, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{}
	where := []string{"1=1"}
	if q.FeedID != "" {
		where = append(where, "a.feed_id = ?")
		args = append(args, q.FeedID)
	}
	if q.FolderID != "" {
		where = append(where, "a.feed_id IN (SELECT feed_id FROM feed_folders WHERE folder_id = ?)")
		args = append(args, q.FolderID)
	}
	if q.UnreadOnly {
		where = append(where, "a.is_read = 0")
	}
	if q.StarredOnly {
		where = append(where, "a.is_starred = 1")
	}
	if q.ReadLaterOnly {
		where = append(where, "a.is_read_later = 1")
	} else if q.ExcludeReadLater {
		where = append(where, "a.is_read_later = 0")
	}
	if q.ArchivedOnly {
		where = append(where, "a.archived_at IS NOT NULL")
	} else if q.ExcludeArchived {
		where = append(where, "a.archived_at IS NULL")
	}
	if q.Search != "" {
		where = append(where, `a.rowid IN (SELECT rowid FROM articles_fts WHERE articles_fts MATCH ?)`)
		args = append(args, sanitizeFTS(q.Search))
	}
	if q.Since != nil {
		where = append(where, `COALESCE(a.published_at, a.discovered_at) >= ?`)
		args = append(args, q.Since.UTC().Format(time.RFC3339Nano))
	}
	order := "COALESCE(a.published_at, a.discovered_at) DESC"
	if q.DefaultSort == "oldest" {
		order = "COALESCE(a.published_at, a.discovered_at) ASC"
	}
	if q.Cursor != "" {
		ts, id, err := decodeCursor(q.Cursor)
		if err == nil {
			if q.DefaultSort == "oldest" {
				where = append(where, `(COALESCE(a.published_at, a.discovered_at) > ? OR (COALESCE(a.published_at, a.discovered_at) = ? AND a.id > ?))`)
			} else {
				where = append(where, `(COALESCE(a.published_at, a.discovered_at) < ? OR (COALESCE(a.published_at, a.discovered_at) = ? AND a.id < ?))`)
			}
			args = append(args, ts, ts, id)
		}
	}
	query := fmt.Sprintf(`%s WHERE %s ORDER BY %s, a.id DESC LIMIT ?`, articleSelect, strings.Join(where, " AND "), order)
	args = append(args, limit+1)
	rows, err := r.db.SQL.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ArticleListResult{}, err
	}
	defer rows.Close()
	var articles []domain.Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return domain.ArticleListResult{}, err
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return domain.ArticleListResult{}, err
	}
	var next string
	if len(articles) > limit {
		last := articles[limit-1]
		articles = articles[:limit]
		ts := last.DiscoveredAt
		if last.PublishedAt != nil {
			ts = *last.PublishedAt
		}
		next = encodeCursor(ts, last.ID)
	}
	return domain.ArticleListResult{Articles: ensureArticles(articles), NextCursor: next}, nil
}

func ensureArticles(articles []domain.Article) []domain.Article {
	if articles == nil {
		return []domain.Article{}
	}
	return articles
}

func (r *ArticleRepo) Get(ctx context.Context, id string) (*domain.Article, error) {
	row := r.db.SQL.QueryRowContext(ctx, articleSelect+` WHERE a.id = ?`, id)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ArticleRepo) ListIDsSince(ctx context.Context, since time.Time) ([]string, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT id FROM articles
		WHERE COALESCE(published_at, discovered_at) >= ?
		ORDER BY COALESCE(published_at, discovered_at) DESC`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ArticleRepo) ListMissedIDs(ctx context.Context) ([]string, error) {
	// Never successfully triaged (priority still none), plus queue failures.
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT a.id FROM articles a
		WHERE a.priority = '' OR a.priority = 'none'
		   OR a.id IN (SELECT article_id FROM ai_queue WHERE status = 'failed')
		ORDER BY COALESCE(a.published_at, a.discovered_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ArticleRepo) UpsertMany(ctx context.Context, articles []domain.Article) (int, error) {
	inserted := 0
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO articles (
			id, feed_id, title, url, author, content, summary,
			published_at, updated_at, external_id, fingerprint, is_read, is_starred,
			discovered_at, priority, rss_content, crawl_status, is_read_later
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'none', ?)
		ON CONFLICT(feed_id, fingerprint) DO UPDATE SET
			title=excluded.title,
			url=CASE WHEN excluded.url != '' THEN excluded.url ELSE articles.url END,
			author=excluded.author,
			content=CASE WHEN excluded.content != '' THEN excluded.content ELSE articles.content END,
			summary=CASE WHEN excluded.summary != '' THEN excluded.summary ELSE articles.summary END,
			rss_content=CASE WHEN excluded.rss_content != '' THEN excluded.rss_content ELSE articles.rss_content END,
			published_at=COALESCE(excluded.published_at, articles.published_at),
			updated_at=excluded.updated_at,
			external_id=CASE WHEN excluded.external_id != '' THEN excluded.external_id ELSE articles.external_id END
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, a := range articles {
		pri := a.Priority
		if pri == "" {
			pri = domain.PriorityNone
		}
		rssContent := a.RSSContent
		if rssContent == "" {
			rssContent = a.Content
		}
		content := a.Content
		if content == "" {
			content = rssContent
		}
		res, err := stmt.ExecContext(ctx,
			a.ID, a.FeedID, a.Title, a.URL, a.Author, content, a.Summary,
			nullTime(a.PublishedAt), nullTime(a.UpdatedAt), a.ExternalID, aFingerprint(a),
			boolToInt(a.IsRead), boolToInt(a.IsStarred), a.DiscoveredAt.UTC().Format(time.RFC3339Nano), string(pri),
			rssContent, boolToInt(a.IsReadLater),
		)
		if err != nil {
			return inserted, err
		}
		n, _ := res.RowsAffected()
		if n == 1 {
			inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

func aFingerprint(a domain.Article) string {
	if a.ExternalID != "" {
		return "guid:" + a.ExternalID
	}
	if a.URL != "" {
		return "url:" + a.URL
	}
	return "fp:" + a.Title + "|" + nullTimeString(a.PublishedAt)
}

func nullTimeString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (r *ArticleRepo) Update(ctx context.Context, article *domain.Article) error {
	pri := article.Priority
	if pri == "" {
		pri = domain.PriorityNone
	}
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE articles SET is_read=?, is_starred=?, title=?, content=?, summary=?, priority=?, story_id=? WHERE id=?`,
		boolToInt(article.IsRead), boolToInt(article.IsStarred), article.Title, article.Content, article.Summary,
		string(pri), nullString(article.StoryID), article.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ArticleRepo) SetCrawlResult(ctx context.Context, id string, status domain.CrawlStatus, crawled string, errMsg string, unreliable bool) error {
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE articles SET crawl_status=?, crawled_content=?, crawl_error=?, crawl_unreliable=? WHERE id=?`,
		string(status), crawled, errMsg, boolToInt(unreliable), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ArticleRepo) SetLiveContent(ctx context.Context, id string, live string) error {
	res, err := r.db.SQL.ExecContext(ctx, `UPDATE articles SET live_content=? WHERE id=?`, live, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ArticleRepo) SetArchived(ctx context.Context, id string, archived bool) error {
	var archivedAt any
	if archived {
		archivedAt = time.Now().UTC().Format(time.RFC3339Nano)
	} else {
		archivedAt = nil
	}
	res, err := r.db.SQL.ExecContext(ctx, `UPDATE articles SET archived_at=? WHERE id=? AND is_read_later=1`, archivedAt, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ArticleRepo) ListNeedingCrawl(ctx context.Context, limit int) ([]domain.Article, error) {
	if limit <= 0 {
		limit = 50
	}
	query := articleSelect + `
		WHERE a.url != ''
		  AND (
		    a.crawl_status IN ('none', 'pending')
		    OR (a.crawl_unreliable = 0 AND a.crawl_status = 'failed' AND a.crawled_content = '')
		  )
		ORDER BY CASE WHEN a.crawl_status IN ('none', 'pending') THEN 0 ELSE 1 END,
		         a.discovered_at ASC
		LIMIT ?`
	rows, err := r.db.SQL.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ArticleRepo) SetPriority(ctx context.Context, id string, priority domain.Priority) error {
	res, err := r.db.SQL.ExecContext(ctx, `UPDATE articles SET priority=? WHERE id=?`, string(priority), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ArticleRepo) FindByExternalKey(ctx context.Context, feedID, externalID, url, fingerprint string) (*domain.Article, error) {
	row := r.db.SQL.QueryRowContext(ctx, articleSelect+`
		WHERE a.feed_id = ? AND (a.fingerprint = ? OR (? != '' AND a.external_id = ?) OR (? != '' AND a.url = ?))
		LIMIT 1`, feedID, fingerprint, externalID, externalID, url, url)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ArticleRepo) SearchCompact(ctx context.Context, query string, limit int) ([]domain.Article, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	q := domain.ArticleQuery{Search: query, Limit: limit}
	res, err := r.List(ctx, q)
	if err != nil {
		return nil, err
	}
	return res.Articles, nil
}

func scanArticle(row rowScanner) (domain.Article, error) {
	var a domain.Article
	var published, updated, archived sql.NullString
	var isRead, isStarred, crawlUnreliable, isReadLater int
	var priority, storyID, crawlStatus, discovered string
	err := row.Scan(
		&a.ID, &a.FeedID, &a.Title, &a.URL, &a.Author, &a.Content, &a.Summary,
		&published, &updated, &a.ExternalID, &isRead, &isStarred,
		&priority, &storyID,
		&a.RSSContent, &a.CrawledContent, &a.LiveContent, &crawlStatus,
		&a.CrawlError, &crawlUnreliable, &isReadLater, &archived,
		&discovered, &a.FeedTitle,
	)
	if err != nil {
		return a, err
	}
	a.PublishedAt = parseTimePtr(published)
	a.UpdatedAt = parseTimePtr(updated)
	a.ArchivedAt = parseTimePtr(archived)
	a.IsRead = isRead == 1
	a.IsStarred = isStarred == 1
	a.Priority = domain.Priority(priority)
	if a.Priority == "" {
		a.Priority = domain.PriorityNone
	}
	a.StoryID = storyID
	a.CrawlStatus = domain.CrawlStatus(crawlStatus)
	if a.CrawlStatus == "" {
		a.CrawlStatus = domain.CrawlNone
	}
	a.CrawlUnreliable = crawlUnreliable == 1
	a.IsReadLater = isReadLater == 1
	if a.Content == "" && a.RSSContent != "" {
		a.Content = a.RSSContent
	}
	a.DiscoveredAt = mustParseTime(discovered)
	return a, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func sanitizeFTS(q string) string {
	parts := strings.Fields(q)
	for i, p := range parts {
		p = strings.ReplaceAll(p, `"`, "")
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, " ")
}

func encodeCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(c string) (string, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("bad cursor")
	}
	return parts[0], parts[1], nil
}
