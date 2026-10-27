package serverstore

import (
	"context"
	"time"
)

// PushFeedOwners applies the same current membership/enabled boundary as the feed reader.
func (s *Store) PushFeedOwners(ctx context.Context, feedID string) (map[string]bool, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `SELECT user_id FROM user_feeds WHERE feed_id=? AND present=1 AND enabled=1`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := map[string]bool{}
	for rows.Next() {
		var owner string
		if err = rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners[owner] = true
	}
	return owners, rows.Err()
}

func (s *Store) PushIdentityMatches(ctx context.Context, owner, issuer, subject string) (bool, error) {
	var n int
	err := s.db.SQL.QueryRowContext(ctx, `SELECT COUNT(1) FROM federated_identities WHERE user_id=? AND issuer=? AND subject=?`, owner, issuer, subject).Scan(&n)
	return n == 1, err
}

// PushFeedsReady closes a user's refresh round only after every currently enabled
// feed has successfully refreshed since the previous daily notification (or opt-in).
func (s *Store) PushFeedsReady(ctx context.Context, owner string, since time.Time) (bool, error) {
	var total, stale int
	err := s.db.SQL.QueryRowContext(ctx, `SELECT COUNT(1),COALESCE(SUM(CASE WHEN f.last_success_at IS NULL OR julianday(f.last_success_at)<julianday(?) THEN 1 ELSE 0 END),0) FROM user_feeds uf JOIN feeds f ON f.id=uf.feed_id WHERE uf.user_id=? AND uf.present=1 AND uf.enabled=1 AND f.is_read_later=0`, since.UTC().Format(time.RFC3339Nano), owner).Scan(&total, &stale)
	return total > 0 && stale == 0, err
}
