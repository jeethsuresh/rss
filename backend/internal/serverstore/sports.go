package serverstore

import (
	"context"
	"sort"
	"strconv"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Store) FollowedTeams(ctx context.Context, userID, sport string) ([]string, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT team_id FROM user_sports_followed_teams
		WHERE user_id=? AND sport=? ORDER BY team_id`, userID, sport)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) SetFollowedTeams(ctx context.Context, userID, sport string, teamIDs []string) ([]string, error) {
	if sport == "" || len(teamIDs) > 500 {
		return nil, domain.ErrInvalidParams
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(teamIDs))
	for _, id := range teamIDs {
		if id == "" || seen[id] {
			continue
		}
		if sport == "mlb" {
			n, err := strconv.Atoi(id)
			if err != nil || n <= 0 {
				return nil, domain.ErrInvalidParams
			}
		}
		seen[id] = true
		clean = append(clean, id)
	}
	previous, err := s.FollowedTeams(ctx, userID, sport)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_sports_followed_teams WHERE user_id=? AND sport=?`, userID, sport); err != nil {
		return nil, err
	}
	for _, id := range clean {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_sports_followed_teams(user_id, sport, team_id, created_at)
			VALUES (?, ?, ?, ?)`, userID, sport, id, formatTime(s.now())); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	sort.Strings(clean)
	for _, id := range previous {
		if !seen[id] {
			if err := s.AppendState(ctx, userID, "sports_team", sportsTeamKey(sport, id), map[string]any{
				"sport": sport, "teamId": id,
			}, false); err != nil {
				return nil, err
			}
		}
	}
	previousSet := map[string]bool{}
	for _, id := range previous {
		previousSet[id] = true
	}
	for _, id := range clean {
		if !previousSet[id] {
			if err := s.AppendState(ctx, userID, "sports_team", sportsTeamKey(sport, id), map[string]any{
				"sport": sport, "teamId": id,
			}, true); err != nil {
				return nil, err
			}
		}
	}
	return clean, nil
}
