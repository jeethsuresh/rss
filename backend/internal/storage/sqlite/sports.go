package sqlite

import (
	"context"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type SportsRepo struct{ db *DB }

func NewSportsRepo(db *DB) *SportsRepo { return &SportsRepo{db: db} }

func (r *SportsRepo) GetFollowedTeamIDs(ctx context.Context) ([]int, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT team_id FROM sports_followed_teams ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *SportsRepo) SetFollowedTeamIDs(ctx context.Context, ids []int) error {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sports_followed_teams`); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO sports_followed_teams(team_id, created_at) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	seen := map[int]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		if _, err := stmt.ExecContext(ctx, id, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func normalizeDotaProvider(provider string) string {
	if provider == "opendota" {
		return "opendota"
	}
	return "pandascore"
}

func (r *SportsRepo) GetDotaFollowedTeamIDs(ctx context.Context, provider string) ([]int, error) {
	provider = normalizeDotaProvider(provider)
	rows, err := r.db.SQL.QueryContext(ctx,
		`SELECT team_id FROM sports_dota_followed_teams WHERE provider = ? ORDER BY created_at ASC`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *SportsRepo) SetDotaFollowedTeamIDs(ctx context.Context, provider string, ids []int) error {
	provider = normalizeDotaProvider(provider)
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sports_dota_followed_teams WHERE provider = ?`, provider); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO sports_dota_followed_teams(provider, team_id, created_at) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	seen := map[int]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		if _, err := stmt.ExecContext(ctx, provider, id, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SportsRepo) GetDotaPinnedEvents(ctx context.Context, provider string) ([]domain.DotaPinnedEvent, error) {
	provider = normalizeDotaProvider(provider)
	rows, err := r.db.SQL.QueryContext(ctx,
		`SELECT event_id, event_type FROM sports_dota_pinned_events WHERE provider = ? ORDER BY created_at ASC`,
		provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DotaPinnedEvent{}
	for rows.Next() {
		var p domain.DotaPinnedEvent
		var typ string
		if err := rows.Scan(&p.EventID, &typ); err != nil {
			return nil, err
		}
		p.EventType = domain.DotaEventType(typ)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SportsRepo) SetDotaPinnedEvents(ctx context.Context, provider string, pins []domain.DotaPinnedEvent) error {
	provider = normalizeDotaProvider(provider)
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sports_dota_pinned_events WHERE provider = ?`, provider); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO sports_dota_pinned_events(provider, event_id, event_type, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	seen := map[string]bool{}
	for _, p := range pins {
		if p.EventID == "" {
			continue
		}
		typ := string(p.EventType)
		if typ == "" {
			typ = string(domain.DotaEventTournament)
		}
		key := typ + ":" + p.EventID
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := stmt.ExecContext(ctx, provider, p.EventID, typ, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
