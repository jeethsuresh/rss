package serverstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Store) AddReadLater(ctx context.Context, userID, rawURL string) (*ReadLaterItem, error) {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}
	now := s.now()
	documentID := uuid.NewString()
	_, err = s.db.SQL.ExecContext(ctx, `
		INSERT INTO web_documents(
			id, normalized_url, url, crawl_status, next_fetch_at, created_at, updated_at
		) VALUES (?, ?, ?, 'pending', ?, ?, ?)
		ON CONFLICT(normalized_url) DO NOTHING`, documentID, normalized, normalized,
		formatTime(now), formatTime(now), formatTime(now))
	if err != nil {
		return nil, err
	}
	if err := s.db.SQL.QueryRowContext(ctx, `SELECT id FROM web_documents WHERE normalized_url=?`, normalized).Scan(&documentID); err != nil {
		return nil, err
	}
	entryID := uuid.NewString()
	_, err = s.db.SQL.ExecContext(ctx, `
		INSERT INTO user_read_later(id, user_id, document_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, document_id) DO UPDATE SET
			archived_at=NULL, updated_at=excluded.updated_at`,
		entryID, userID, documentID, formatTime(now), formatTime(now))
	if err != nil {
		return nil, err
	}
	if err := s.db.SQL.QueryRowContext(ctx, `SELECT id FROM user_read_later WHERE user_id=? AND document_id=?`, userID, documentID).Scan(&entryID); err != nil {
		return nil, err
	}
	item, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	if err := s.appendReadLaterState(ctx, userID, item, true); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) ListReadLater(ctx context.Context, userID, filter, search string, limit int) ([]ReadLaterItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	where := []string{"url.user_id=?"}
	args := []any{userID}
	switch filter {
	case "archived":
		where = append(where, "url.archived_at IS NOT NULL")
	case "unread":
		where = append(where, "url.archived_at IS NULL", "url.is_read=0")
	case "starred":
		where = append(where, "url.archived_at IS NULL", "url.is_starred=1")
	default:
		where = append(where, "url.archived_at IS NULL")
	}
	if strings.TrimSpace(search) != "" {
		where = append(where, "(d.title LIKE ? OR d.url LIKE ?)")
		needle := "%" + strings.TrimSpace(search) + "%"
		args = append(args, needle, needle)
	}
	args = append(args, limit)
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT url.id, d.id, d.url, d.final_url, d.title, d.crawled_content,
		       d.reader_content, d.crawl_status, d.crawl_error, d.fetched_at,
		       url.is_read, url.is_starred, url.archived_at, url.created_at, url.updated_at
		FROM user_read_later url
		JOIN web_documents d ON d.id=url.document_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY url.created_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReadLaterItem{}
	for rows.Next() {
		item, err := scanReadLater(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetReadLater(ctx context.Context, userID, entryID string) (*ReadLaterItem, error) {
	row := s.db.SQL.QueryRowContext(ctx, `
		SELECT url.id, d.id, d.url, d.final_url, d.title, d.crawled_content,
		       d.reader_content, d.crawl_status, d.crawl_error, d.fetched_at,
		       url.is_read, url.is_starred, url.archived_at, url.created_at, url.updated_at
		FROM user_read_later url
		JOIN web_documents d ON d.id=url.document_id
		WHERE url.user_id=? AND url.id=?`, userID, entryID)
	item, err := scanReadLater(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) SetReadLaterState(ctx context.Context, userID, entryID string, patch ReadLaterPatch) (*ReadLaterItem, error) {
	item, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	if patch.IsRead != nil {
		item.IsRead = *patch.IsRead
	}
	if patch.IsStarred != nil {
		item.IsStarred = *patch.IsStarred
	}
	archived := any(nil)
	if item.ArchivedAt != nil {
		archived = formatTime(*item.ArchivedAt)
	}
	if patch.Archived != nil {
		if *patch.Archived {
			t := s.now()
			archived = formatTime(t)
		} else {
			archived = nil
		}
	}
	res, err := s.db.SQL.ExecContext(ctx, `
		UPDATE user_read_later SET is_read=?, is_starred=?, archived_at=?, updated_at=?
		WHERE id=? AND user_id=?`, boolInt(item.IsRead), boolInt(item.IsStarred), archived,
		formatTime(s.now()), entryID, userID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, domain.ErrNotFound
	}
	result, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	if err := s.appendReadLaterState(ctx, userID, result, true); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) DeleteReadLater(ctx context.Context, userID, entryID string) error {
	item, err := s.GetReadLater(ctx, userID, entryID)
	if err != nil {
		return err
	}
	res, err := s.db.SQL.ExecContext(ctx, `DELETE FROM user_read_later WHERE id=? AND user_id=?`, entryID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return s.appendReadLaterState(ctx, userID, item, false)
}

func (s *Store) appendReadLaterState(ctx context.Context, userID string, item *ReadLaterItem, present bool) error {
	archivedAt := ""
	if item.ArchivedAt != nil {
		archivedAt = formatTime(*item.ArchivedAt)
	}
	return s.AppendState(ctx, userID, "read_later", item.URL, map[string]any{
		"title": item.Title, "isRead": item.IsRead, "isStarred": item.IsStarred,
		"archivedAt": archivedAt,
	}, present)
}

func scanReadLater(row interface{ Scan(...any) error }) (ReadLaterItem, error) {
	var item ReadLaterItem
	var fetched, archived sql.NullString
	var read, starred int
	var created, updated string
	err := row.Scan(&item.ID, &item.SharedDocumentID, &item.URL, &item.FinalURL, &item.Title,
		&item.CrawledContent, &item.ReaderContent, &item.CrawlStatus, &item.CrawlError, &fetched,
		&read, &starred, &archived, &created, &updated)
	if err != nil {
		return item, err
	}
	item.FetchedAt = parseNullTime(fetched)
	item.ArchivedAt = parseNullTime(archived)
	item.IsRead = read == 1
	item.IsStarred = starred == 1
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	return item, nil
}
