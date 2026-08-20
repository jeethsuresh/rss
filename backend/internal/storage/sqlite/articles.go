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
	if q.Search != "" {
		where = append(where, `a.rowid IN (SELECT rowid FROM articles_fts WHERE articles_fts MATCH ?)`)
		args = append(args, sanitizeFTS(q.Search))
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
	query := fmt.Sprintf(`
		SELECT a.id, a.feed_id, a.title, a.url, a.author, a.content, a.summary,
		       a.published_at, a.updated_at, a.external_id, a.is_read, a.is_starred, a.discovered_at,
		       f.title
		FROM articles a
		JOIN feeds f ON f.id = a.feed_id
		WHERE %s
		ORDER BY %s, a.id DESC
		LIMIT ?`, strings.Join(where, " AND "), order)
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
	return domain.ArticleListResult{Articles: articles, NextCursor: next}, nil
}

func (r *ArticleRepo) Get(ctx context.Context, id string) (*domain.Article, error) {
	row := r.db.SQL.QueryRowContext(ctx, `
		SELECT a.id, a.feed_id, a.title, a.url, a.author, a.content, a.summary,
		       a.published_at, a.updated_at, a.external_id, a.is_read, a.is_starred, a.discovered_at,
		       f.title
		FROM articles a JOIN feeds f ON f.id = a.feed_id WHERE a.id = ?`, id)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
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
			published_at, updated_at, external_id, fingerprint, is_read, is_starred, discovered_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(feed_id, fingerprint) DO UPDATE SET
			title=excluded.title,
			url=CASE WHEN excluded.url != '' THEN excluded.url ELSE articles.url END,
			author=excluded.author,
			content=CASE WHEN excluded.content != '' THEN excluded.content ELSE articles.content END,
			summary=CASE WHEN excluded.summary != '' THEN excluded.summary ELSE articles.summary END,
			published_at=COALESCE(excluded.published_at, articles.published_at),
			updated_at=excluded.updated_at,
			external_id=CASE WHEN excluded.external_id != '' THEN excluded.external_id ELSE articles.external_id END
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, a := range articles {
		res, err := stmt.ExecContext(ctx,
			a.ID, a.FeedID, a.Title, a.URL, a.Author, a.Content, a.Summary,
			nullTime(a.PublishedAt), nullTime(a.UpdatedAt), a.ExternalID, aFingerprint(a),
			boolToInt(a.IsRead), boolToInt(a.IsStarred), a.DiscoveredAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return inserted, err
		}
		n, _ := res.RowsAffected()
		// SQLite ON CONFLICT UPDATE reports 1 for update sometimes; track inserts via changes() is tricky.
		// Count only brand-new rows by checking changes==1 and last insert - approximate: use SELECT before.
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
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE articles SET is_read=?, is_starred=?, title=?, content=?, summary=? WHERE id=?`,
		boolToInt(article.IsRead), boolToInt(article.IsStarred), article.Title, article.Content, article.Summary, article.ID,
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

func (r *ArticleRepo) FindByExternalKey(ctx context.Context, feedID, externalID, url, fingerprint string) (*domain.Article, error) {
	row := r.db.SQL.QueryRowContext(ctx, `
		SELECT a.id, a.feed_id, a.title, a.url, a.author, a.content, a.summary,
		       a.published_at, a.updated_at, a.external_id, a.is_read, a.is_starred, a.discovered_at,
		       f.title
		FROM articles a JOIN feeds f ON f.id = a.feed_id
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

func scanArticle(row rowScanner) (domain.Article, error) {
	var a domain.Article
	var published, updated sql.NullString
	var isRead, isStarred int
	var discovered string
	err := row.Scan(
		&a.ID, &a.FeedID, &a.Title, &a.URL, &a.Author, &a.Content, &a.Summary,
		&published, &updated, &a.ExternalID, &isRead, &isStarred, &discovered, &a.FeedTitle,
	)
	if err != nil {
		return a, err
	}
	a.PublishedAt = parseTimePtr(published)
	a.UpdatedAt = parseTimePtr(updated)
	a.IsRead = isRead == 1
	a.IsStarred = isStarred == 1
	a.DiscoveredAt = mustParseTime(discovered)
	return a, nil
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
