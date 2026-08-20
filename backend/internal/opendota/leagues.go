package opendota

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type League struct {
	LeagueID int    `json:"leagueid"`
	Name     string `json:"name"`
	Tier     string `json:"tier"`
}

type LeagueMatchRow struct {
	MatchID        int64  `json:"match_id"`
	StartTime      int64  `json:"start_time"`
	Duration       int    `json:"duration"`
	RadiantScore   int    `json:"radiant_score"`
	DireScore      int    `json:"dire_score"`
	RadiantWin     bool   `json:"radiant_win"`
	RadiantTeamID  int    `json:"radiant_team_id"`
	DireTeamID     int    `json:"dire_team_id"`
	RadiantTeamName string `json:"radiant_team_name"`
	DireTeamName   string `json:"dire_team_name"`
	SeriesID       int64  `json:"series_id"`
	SeriesType     int    `json:"series_type"`
	LeagueID       int    `json:"leagueid"`
}

type LeagueTeam struct {
	TeamID  int    `json:"team_id"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	LogoURL string `json:"logo_url"`
}

var yearInName = regexp.MustCompile(`(?:^|[^\d])(20\d{2})(?:[^\d]|$)`)

func (c *Client) ListLeagues(ctx context.Context) ([]League, error) {
	var list []League
	if err := c.getJSON(ctx, "/leagues", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) LeagueMatches(ctx context.Context, leagueID int) ([]LeagueMatchRow, error) {
	var list []LeagueMatchRow
	if err := c.getJSON(ctx, fmt.Sprintf("/leagues/%d/matches", leagueID), &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) LeagueTeams(ctx context.Context, leagueID int) ([]LeagueTeam, error) {
	var list []LeagueTeam
	if err := c.getJSON(ctx, fmt.Sprintf("/leagues/%d/teams", leagueID), &list); err != nil {
		return nil, err
	}
	return list, nil
}

func LeagueYear(name string) int {
	ms := yearInName.FindStringSubmatch(name)
	if len(ms) < 2 {
		return 0
	}
	y, _ := strconv.Atoi(ms[1])
	return y
}

func MapLeagueTier(raw string) domain.DotaEventTier {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "premium":
		return domain.DotaTierPremier
	case "professional":
		return domain.DotaTierProfessional
	case "semipro", "semi-pro", "amateur":
		return domain.DotaTierAmateur
	default:
		return domain.DotaTierUnknown
	}
}

func tierRank(t string) int {
	switch strings.ToLower(t) {
	case "premium":
		return 3
	case "professional":
		return 2
	default:
		return 1
	}
}

// EventsForYear filters leagues for a calendar year. Recent/premium leagues first.
func EventsForYear(leagues []League, year int) []domain.DotaEvent {
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	type scored struct {
		ev    domain.DotaEvent
		rank  int
		name  string
	}
	var scoredList []scored
	yearStr := strconv.Itoa(year)
	for _, l := range leagues {
		if l.LeagueID <= 0 || strings.TrimSpace(l.Name) == "" {
			continue
		}
		y := LeagueYear(l.Name)
		if y != year && !strings.Contains(l.Name, yearStr) {
			continue
		}
		if y == 0 {
			y = year
		}
		ev := domain.DotaEvent{
			ID:         strconv.Itoa(l.LeagueID),
			Name:       strings.TrimSpace(l.Name),
			Type:       domain.DotaEventTournament,
			Tier:       MapLeagueTier(l.Tier),
			Status:     domain.DotaEventOngoing,
			LeagueID:   strconv.Itoa(l.LeagueID),
			LeagueName: strings.TrimSpace(l.Name),
			Year:       y,
			Organizer:  strings.TrimSpace(l.Name),
		}
		scoredList = append(scoredList, scored{ev: ev, rank: tierRank(l.Tier), name: ev.Name})
	}
	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].rank != scoredList[j].rank {
			return scoredList[i].rank > scoredList[j].rank
		}
		return scoredList[i].name < scoredList[j].name
	})
	out := make([]domain.DotaEvent, 0, len(scoredList))
	for _, s := range scoredList {
		out = append(out, s.ev)
	}
	return out
}

func teamName(id int, fallback string, byID map[int]LeagueTeam) string {
	if t, ok := byID[id]; ok && t.Name != "" {
		return t.Name
	}
	if fallback != "" {
		return fallback
	}
	if id > 0 {
		return fmt.Sprintf("Team %d", id)
	}
	return "TBD"
}

func teamShort(id int, byID map[int]LeagueTeam) string {
	if t, ok := byID[id]; ok {
		return t.Tag
	}
	return ""
}

func teamLogo(id int, byID map[int]LeagueTeam) string {
	if t, ok := byID[id]; ok {
		return t.LogoURL
	}
	return ""
}

// SeriesFromLeagueMatches groups OpenDota league matches into BO series (DotaMatch).
func SeriesFromLeagueMatches(rows []LeagueMatchRow, teams []LeagueTeam, year int) []domain.DotaMatch {
	byID := map[int]LeagueTeam{}
	for _, t := range teams {
		byID[t.TeamID] = t
	}
	type group struct {
		key  int64
		rows []LeagueMatchRow
	}
	order := []int64{}
	groups := map[int64]*group{}
	for _, r := range rows {
		key := r.SeriesID
		if key == 0 {
			key = r.MatchID
		}
		g, ok := groups[key]
		if !ok {
			g = &group{key: key}
			groups[key] = g
			order = append(order, key)
		}
		g.rows = append(g.rows, r)
	}
	out := make([]domain.DotaMatch, 0, len(order))
	for _, key := range order {
		g := groups[key]
		sort.SliceStable(g.rows, func(i, j int) bool {
			return g.rows[i].StartTime < g.rows[j].StartTime
		})
		first := g.rows[0]
		last := g.rows[len(g.rows)-1]
		aID, bID := first.RadiantTeamID, first.DireTeamID
		// Prefer consistent team pair across series
		a := domain.DotaTeam{
			ID: aID, Name: teamName(aID, first.RadiantTeamName, byID),
			ShortName: teamShort(aID, byID), LogoURL: teamLogo(aID, byID),
		}
		b := domain.DotaTeam{
			ID: bID, Name: teamName(bID, first.DireTeamName, byID),
			ShortName: teamShort(bID, byID), LogoURL: teamLogo(bID, byID),
		}
		scoreA, scoreB := 0, 0
		for _, r := range g.rows {
			if r.RadiantWin {
				if r.RadiantTeamID == aID {
					scoreA++
				} else if r.RadiantTeamID == bID {
					scoreB++
				}
			} else {
				if r.DireTeamID == aID {
					scoreA++
				} else if r.DireTeamID == bID {
					scoreB++
				}
			}
		}
		sa, sb := scoreA, scoreB
		start := time.Unix(first.StartTime, 0).UTC().Format(time.RFC3339)
		status := domain.DotaMatchCompleted
		// Heuristic: last game finished recently and series incomplete → live-ish; keep completed for OD historical.
		if last.Duration <= 0 {
			status = domain.DotaMatchLive
		}
		bo := len(g.rows)
		bestOf := &bo
		out = append(out, domain.DotaMatch{
			ID:          int(key),
			EventID:     strconv.Itoa(first.LeagueID),
			TeamA:       a,
			TeamB:       b,
			ScheduledAt: &start,
			Status:      status,
			BestOf:      bestOf,
			ScoreA:      &sa,
			ScoreB:      &sb,
			Year:        year,
		})
	}
	// Newest series first
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := "", ""
		if out[i].ScheduledAt != nil {
			ai = *out[i].ScheduledAt
		}
		if out[j].ScheduledAt != nil {
			aj = *out[j].ScheduledAt
		}
		return ai > aj
	})
	return out
}

// GamesForSeries returns individual Steam matches as DotaGame stubs for a series key.
func GamesForSeries(rows []LeagueMatchRow, seriesKey int64, teams []LeagueTeam) []domain.DotaGame {
	byID := map[int]LeagueTeam{}
	for _, t := range teams {
		byID[t.TeamID] = t
	}
	var filtered []LeagueMatchRow
	for _, r := range rows {
		key := r.SeriesID
		if key == 0 {
			key = r.MatchID
		}
		if key == seriesKey {
			filtered = append(filtered, r)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].StartTime < filtered[j].StartTime
	})
	out := make([]domain.DotaGame, 0, len(filtered))
	for i, r := range filtered {
		dur := r.Duration
		start := time.Unix(r.StartTime, 0).UTC().Format(time.RFC3339)
		sid := r.MatchID
		winnerName := ""
		if r.RadiantWin {
			winnerName = teamName(r.RadiantTeamID, r.RadiantTeamName, byID)
		} else {
			winnerName = teamName(r.DireTeamID, r.DireTeamName, byID)
		}
		side := domain.DotaDire
		if r.RadiantWin {
			side = domain.DotaRadiant
		}
		a := domain.DotaTeam{ID: r.RadiantTeamID, Name: teamName(r.RadiantTeamID, r.RadiantTeamName, byID), ShortName: teamShort(r.RadiantTeamID, byID)}
		b := domain.DotaTeam{ID: r.DireTeamID, Name: teamName(r.DireTeamID, r.DireTeamName, byID), ShortName: teamShort(r.DireTeamID, byID)}
		rs, ds := r.RadiantScore, r.DireScore
		out = append(out, domain.DotaGame{
			ID:                strconv.FormatInt(r.MatchID, 10),
			MatchID:           int(seriesKey),
			GameIndex:         i + 1,
			StartedAt:         &start,
			DurationSeconds:   &dur,
			Winner:            side,
			WinnerTeamName:    winnerName,
			RadiantTeam:       &a,
			DireTeam:          &b,
			RadiantScore:      &rs,
			DireScore:         &ds,
			StratzMatchID:     &sid,
			MappingConfidence: "exact",
			DetailAvailable:   false,
		})
	}
	return out
}
