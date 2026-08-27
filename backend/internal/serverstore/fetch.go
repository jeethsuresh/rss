package serverstore

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

func (s *Store) ClaimDueFeed(ctx context.Context, owner string, lease time.Duration) (*FeedClaim, error) {
	now := s.now()
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var feedID string
	err = tx.QueryRowContext(ctx, `
		SELECT fs.feed_id
		FROM feed_fetch_state fs
		JOIN feeds f ON f.id=fs.feed_id
		WHERE f.enabled=1 AND f.is_read_later=0
		  AND fs.next_fetch_at <= ?
		  AND (fs.lease_until IS NULL OR fs.lease_until <= ?)
		  AND EXISTS (
		    SELECT 1 FROM user_feeds uf
		    WHERE uf.feed_id=fs.feed_id AND uf.present=1 AND uf.enabled=1
		  )
		ORDER BY fs.next_fetch_at ASC
		LIMIT 1`, formatTime(now), formatTime(now)).Scan(&feedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE feed_fetch_state SET lease_until=?, lease_owner=?, updated_at=?
		WHERE feed_id=? AND (lease_until IS NULL OR lease_until <= ?)`,
		formatTime(now.Add(lease)), owner, formatTime(now), feedID, formatTime(now))
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &FeedClaim{FeedID: feedID, Owner: owner}, nil
}

type FetchOutcome struct {
	NewItems    int
	NotModified bool
	Duration    time.Duration
	Err         error
}

func (s *Store) CompleteFeedFetch(ctx context.Context, claim FeedClaim, outcome FetchOutcome) (time.Time, error) {
	now := s.now()
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var avg float64
	var lastChanged sql.NullString
	var unchanged, failures int
	err = tx.QueryRowContext(ctx, `
		SELECT average_item_interval_seconds, last_changed_at,
		       consecutive_unchanged, consecutive_failures
		FROM feed_fetch_state WHERE feed_id=? AND lease_owner=?`, claim.FeedID, claim.Owner).
		Scan(&avg, &lastChanged, &unchanged, &failures)
	if err != nil {
		return time.Time{}, err
	}
	last := parseNullTime(lastChanged)
	nextDelay, nextAvg, nextLast, nextUnchanged, nextFailures := adaptiveDelay(
		now, avg, last, unchanged, failures, outcome.NewItems, outcome.Err != nil)
	next := now.Add(nextDelay)
	errText := ""
	if outcome.Err != nil {
		errText = outcome.Err.Error()
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE feed_fetch_state SET
			next_fetch_at=?, lease_until=NULL, lease_owner='',
			average_item_interval_seconds=?, last_changed_at=?,
			consecutive_unchanged=?, consecutive_failures=?,
			last_new_item_count=?, updated_at=?
		WHERE feed_id=? AND lease_owner=?`,
		formatTime(next), nextAvg, nullTimeValue(nextLast), nextUnchanged, nextFailures,
		outcome.NewItems, formatTime(now), claim.FeedID, claim.Owner)
	if err != nil {
		return time.Time{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return time.Time{}, errors.New("feed fetch lease lost")
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO feed_fetch_log(feed_id, fetched_at, duration_ms, new_item_count, not_modified, error)
		VALUES (?, ?, ?, ?, ?, ?)`, claim.FeedID, formatTime(now), outcome.Duration.Milliseconds(),
		outcome.NewItems, boolInt(outcome.NotModified), errText)
	if err != nil {
		return time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return time.Time{}, err
	}
	return next, nil
}

func adaptiveDelay(now time.Time, averageSeconds float64, lastChanged *time.Time, unchanged, failures, newItems int, failed bool) (time.Duration, float64, *time.Time, int, int) {
	if averageSeconds <= 0 {
		averageSeconds = 3600
	}
	if failed {
		failures++
		hours := math.Pow(2, float64(minInt(failures-1, 5)))
		delay := clampDuration(time.Duration(hours*float64(time.Hour)), 15*time.Minute, 24*time.Hour)
		return delay, averageSeconds, lastChanged, unchanged, failures
	}
	failures = 0
	if newItems > 0 {
		observed := 1800.0
		if lastChanged != nil && now.After(*lastChanged) {
			observed = now.Sub(*lastChanged).Seconds() / float64(newItems)
		}
		observed = math.Max(300, math.Min(observed, 24*3600))
		averageSeconds = 0.7*averageSeconds + 0.3*observed
		delay := clampDuration(time.Duration(averageSeconds*0.75)*time.Second, 5*time.Minute, 12*time.Hour)
		changed := now
		return delay, averageSeconds, &changed, 0, 0
	}
	unchanged++
	multiplier := math.Pow(1.45, float64(minInt(unchanged, 8)))
	delay := clampDuration(time.Duration(averageSeconds*multiplier)*time.Second, 15*time.Minute, 24*time.Hour)
	return delay, averageSeconds, lastChanged, unchanged, 0
}

func clampDuration(v, low, high time.Duration) time.Duration {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nullTimeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func (s *Store) ClaimDueDocument(ctx context.Context, owner string, lease time.Duration) (*DocumentClaim, error) {
	now := s.now()
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var claim DocumentClaim
	err = tx.QueryRowContext(ctx, `
		SELECT id, url FROM web_documents
		WHERE crawl_status IN ('pending', 'failed') AND next_fetch_at <= ?
		  AND (lease_until IS NULL OR lease_until <= ?)
		ORDER BY next_fetch_at ASC LIMIT 1`, formatTime(now), formatTime(now)).Scan(&claim.DocumentID, &claim.URL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE web_documents SET lease_until=?, lease_owner=?, crawl_status='pending', updated_at=?
		WHERE id=? AND (lease_until IS NULL OR lease_until <= ?)`,
		formatTime(now.Add(lease)), owner, formatTime(now), claim.DocumentID, formatTime(now))
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil
	}
	claim.Owner = owner
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &claim, nil
}

func (s *Store) CompleteDocument(ctx context.Context, claim DocumentClaim, finalURL, title, crawled, reader string, fetchErr error) error {
	now := s.now()
	status := "ok"
	errText := ""
	fetchedAt := any(formatTime(now))
	next := now.Add(100 * 365 * 24 * time.Hour)
	if fetchErr != nil {
		status = "failed"
		errText = fetchErr.Error()
		fetchedAt = nil
		next = now.Add(24 * time.Hour)
	}
	res, err := s.db.SQL.ExecContext(ctx, `
		UPDATE web_documents SET final_url=?, title=?, crawled_content=?, reader_content=?,
			crawl_status=?, crawl_error=?, fetched_at=?, next_fetch_at=?,
			lease_until=NULL, lease_owner='', updated_at=?
		WHERE id=? AND lease_owner=?`, finalURL, title, crawled, reader, status, errText,
		fetchedAt, formatTime(next), formatTime(now), claim.DocumentID, claim.Owner)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("document fetch lease lost")
	}
	return nil
}
