package opendota

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type TeamDetail struct {
	TeamID  int    `json:"team_id"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	LogoURL string `json:"logo_url"`
}

type TeamMatchRow struct {
	MatchID          int64  `json:"match_id"`
	StartTime        int64  `json:"start_time"`
	Duration         int    `json:"duration"`
	Radiant          bool   `json:"radiant"`
	RadiantWin       bool   `json:"radiant_win"`
	RadiantScore     int    `json:"radiant_score"`
	DireScore        int    `json:"dire_score"`
	LeagueID         int    `json:"leagueid"`
	LeagueName       string `json:"league_name"`
	OpposingTeamID   int    `json:"opposing_team_id"`
	OpposingTeamName string `json:"opposing_team_name"`
	OpposingTeamLogo string `json:"opposing_team_logo"`
}

func (c *Client) GetTeam(ctx context.Context, teamID int) (*domain.DotaTeam, error) {
	if teamID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	var raw TeamDetail
	if err := c.getJSON(ctx, fmt.Sprintf("/teams/%d", teamID), &raw); err != nil {
		return nil, err
	}
	if raw.TeamID == 0 {
		raw.TeamID = teamID
	}
	return &domain.DotaTeam{
		ID:        raw.TeamID,
		Name:      raw.Name,
		ShortName: raw.Tag,
		LogoURL:   raw.LogoURL,
	}, nil
}

func (c *Client) SearchTeams(ctx context.Context, query string) ([]domain.DotaTeam, error) {
	q := strings.TrimSpace(strings.ToLower(query))
	if len(q) < 2 {
		return []domain.DotaTeam{}, nil
	}
	var list []odTeam
	if err := c.getJSON(ctx, "/teams", &list); err != nil {
		return nil, err
	}
	out := make([]domain.DotaTeam, 0, 25)
	for _, t := range list {
		if t.TeamID <= 0 {
			continue
		}
		name := strings.ToLower(t.Name)
		tag := strings.ToLower(t.Tag)
		if !strings.Contains(name, q) && !strings.Contains(tag, q) {
			continue
		}
		out = append(out, domain.DotaTeam{
			ID:        t.TeamID,
			Name:      t.Name,
			ShortName: t.Tag,
		})
		if len(out) >= 25 {
			break
		}
	}
	return out, nil
}

func (c *Client) ListTeamMatchesForYear(ctx context.Context, teamID, year int) ([]domain.DotaMatch, error) {
	if teamID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	var list []TeamMatchRow
	if err := c.getJSON(ctx, fmt.Sprintf("/teams/%d/matches", teamID), &list); err != nil {
		return nil, err
	}
	self, _ := c.GetTeam(ctx, teamID)
	selfTeam := domain.DotaTeam{ID: teamID, Name: fmt.Sprintf("Team %d", teamID)}
	if self != nil {
		selfTeam = *self
	}
	out := make([]domain.DotaMatch, 0, 40)
	for _, r := range list {
		if r.MatchID == 0 || r.StartTime <= 0 {
			continue
		}
		st := time.Unix(r.StartTime, 0).UTC()
		if st.Year() != year {
			continue
		}
		opp := domain.DotaTeam{
			ID:      r.OpposingTeamID,
			Name:    r.OpposingTeamName,
			LogoURL: r.OpposingTeamLogo,
		}
		if opp.Name == "" {
			opp.Name = fmt.Sprintf("Team %d", r.OpposingTeamID)
		}
		teamA, teamB := selfTeam, opp
		scoreA, scoreB := 0, 0
		won := (r.Radiant && r.RadiantWin) || (!r.Radiant && !r.RadiantWin)
		if won {
			scoreA = 1
		} else {
			scoreB = 1
		}
		sa, sb := scoreA, scoreB
		bo := 1
		start := st.Format(time.RFC3339)
		status := domain.DotaMatchCompleted
		if r.Duration <= 0 {
			status = domain.DotaMatchLive
		}
		out = append(out, domain.DotaMatch{
			ID:          int(r.MatchID), // BO1: Steam id as match id
			EventID:     strconv.Itoa(r.LeagueID),
			TeamA:       teamA,
			TeamB:       teamB,
			ScheduledAt: &start,
			Status:      status,
			BestOf:      &bo,
			ScoreA:      &sa,
			ScoreB:      &sb,
			Year:        year,
		})
		if len(out) >= 50 {
			break
		}
	}
	return out, nil
}
