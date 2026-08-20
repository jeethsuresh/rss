package sqlite

import (
	"context"
	"time"
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
