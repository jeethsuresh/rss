package serverstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/crawl"
)

// GetOrClaim implements crawl.SharedDocumentCache. All RSS and Read Later page
// fetches converge on web_documents, so a normalized URL has one stored crawl.
func (s *Store) GetOrClaim(ctx context.Context, rawURL, owner string, lease time.Duration) (crawl.SharedDocument, bool, error) {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return crawl.SharedDocument{}, false, err
	}
	now := s.now()
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return crawl.SharedDocument{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO web_documents(id, normalized_url, url, crawl_status, next_fetch_at, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?)
		ON CONFLICT(normalized_url) DO NOTHING`, uuid.NewString(), normalized, normalized,
		formatTime(now), formatTime(now), formatTime(now))
	if err != nil {
		return crawl.SharedDocument{}, false, err
	}
	var document crawl.SharedDocument
	var leaseUntil, nextFetch sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id, crawl_status, final_url, title, crawled_content, reader_content,
		       crawl_error, lease_until, next_fetch_at
		FROM web_documents WHERE normalized_url=?`, normalized).
		Scan(&document.ID, &document.Status, &document.FinalURL, &document.Title,
			&document.CrawledContent, &document.ReaderContent, &document.Error, &leaseUntil, &nextFetch)
	if err != nil {
		return crawl.SharedDocument{}, false, err
	}
	if document.Status == "ok" {
		return document, false, tx.Commit()
	}
	if until := parseNullTime(leaseUntil); until != nil && until.After(now) {
		return document, false, tx.Commit()
	}
	if next := parseNullTime(nextFetch); document.Status == "failed" && next != nil && next.After(now) {
		return document, false, tx.Commit()
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE web_documents SET lease_until=?, lease_owner=?, crawl_status='pending', updated_at=?
		WHERE id=? AND (lease_until IS NULL OR lease_until <= ?)`,
		formatTime(now.Add(lease)), owner, formatTime(now), document.ID, formatTime(now))
	if err != nil {
		return crawl.SharedDocument{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return document, false, tx.Commit()
	}
	document.Status = "pending"
	if err := tx.Commit(); err != nil {
		return crawl.SharedDocument{}, false, err
	}
	return document, true, nil
}

func (s *Store) Complete(ctx context.Context, documentID, owner, finalURL, title, crawled, reader string, fetchErr error) error {
	return s.CompleteDocument(ctx, DocumentClaim{DocumentID: documentID, Owner: owner}, finalURL, title, crawled, reader, fetchErr)
}

var _ crawl.SharedDocumentCache = (*Store)(nil)
