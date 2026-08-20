package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/opendota"
)

func odEventsKey(year int) string { return fmt.Sprintf("dota.od.events.%d", year) }
func odEventMatchesKey(eventID string) string {
	return fmt.Sprintf("dota.od.event.matches.%s", eventID)
}
func odMatchKey(matchID int) string { return fmt.Sprintf("dota.od.match.%d", matchID) }
func odSeriesMetaKey(matchID int) string {
	return fmt.Sprintf("dota.od.series.meta.%d", matchID)
}
func odTeamKey(teamID int) string { return fmt.Sprintf("dota.od.team.%d", teamID) }
func odTeamMatchesKey(teamID, year int) string {
	return fmt.Sprintf("dota.od.team.matches.%d.%d", teamID, year)
}
func odGameKey(steamID int64) string { return fmt.Sprintf("dota.od.game.%d", steamID) }

type odSeriesMeta struct {
	EventID string `json:"eventId"`
	Year    int    `json:"year"`
}

func (s *Service) sportsDotaStatusOD(ctx context.Context) (domain.DotaProvidersStatus, error) {
	st := s.Sports.dotaConfigured()
	st.Provider = "opendota"
	st.Ready = s.Sports.OpenDota != nil
	return st, nil
}

func (s *Service) sportsDotaEventsOD(ctx context.Context, year int) ([]domain.DotaEvent, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return []domain.DotaEvent{}, nil
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := odEventsKey(year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaEvents, "dota.od.events",
		func(ev []domain.DotaEvent) map[string]any {
			return map[string]any{"year": year, "events": ev}
		},
		func(c context.Context) ([]domain.DotaEvent, error) {
			leagues, err := s.Sports.OpenDota.ListLeagues(c)
			if err != nil {
				return nil, err
			}
			return opendota.EventsForYear(leagues, year), nil
		},
	)
	return out, err
}

func (s *Service) sportsDotaEventMatchesOD(ctx context.Context, eventID string) ([]domain.DotaMatch, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return []domain.DotaMatch{}, nil
	}
	leagueID, err := strconv.Atoi(eventID)
	if err != nil || leagueID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	year := time.Now().UTC().Year()
	key := odEventMatchesKey(eventID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaUpcoming, "dota.od.event.matches",
		func(ms []domain.DotaMatch) map[string]any {
			return map[string]any{"eventId": eventID, "matches": ms}
		},
		func(c context.Context) ([]domain.DotaMatch, error) {
			rows, err := s.Sports.OpenDota.LeagueMatches(c, leagueID)
			if err != nil {
				return nil, err
			}
			teams, _ := s.Sports.OpenDota.LeagueTeams(c, leagueID)
			ms := opendota.SeriesFromLeagueMatches(rows, teams, year)
			for _, m := range ms {
				s.Sports.writeCache(c, odSeriesMetaKey(m.ID), odSeriesMeta{EventID: eventID, Year: year})
				gameRows := opendota.GamesForSeries(rows, int64(m.ID), teams)
				detail := &domain.DotaMatchDetail{Match: m, Games: gameRows, Live: m.Status == domain.DotaMatchLive}
				s.Sports.writeCache(c, odMatchKey(m.ID), detail)
			}
			return ms, nil
		},
	)
	return out, err
}

func (s *Service) sportsDotaMatchGetOD(ctx context.Context, matchID int) (*domain.DotaMatchDetail, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return nil, domain.ErrInvalidParams
	}
	key := odMatchKey(matchID)
	var cached domain.DotaMatchDetail
	if at, ok := s.Sports.readCache(ctx, key, &cached); ok {
		ttl := ttlDotaUpcoming
		switch cached.Match.Status {
		case domain.DotaMatchLive:
			ttl = ttlDotaLive
		case domain.DotaMatchCompleted:
			ttl = ttlDotaDone
		}
		if time.Since(at) <= ttl {
			return &cached, nil
		}
	}
	var meta odSeriesMeta
	if _, ok := s.Sports.readCache(ctx, odSeriesMetaKey(matchID), &meta); ok && meta.EventID != "" {
		_, _ = s.sportsDotaEventMatchesOD(ctx, meta.EventID)
		if _, ok := s.Sports.readCache(ctx, key, &cached); ok {
			return &cached, nil
		}
	}
	// BO1 team-match path: treat matchID as Steam match id.
	g, err := s.Sports.OpenDota.GetMatchDetail(ctx, int64(matchID))
	if err != nil {
		if _, ok := s.Sports.readCache(ctx, key, &cached); ok {
			return &cached, nil
		}
		return nil, err
	}
	g.MatchID = matchID
	g.GameIndex = 1
	g.DetailAvailable = true
	bo := 1
	sa, sb := 0, 0
	if g.Winner == domain.DotaRadiant {
		sa = 1
	} else {
		sb = 1
	}
	teamA := domain.DotaTeam{Name: "Radiant"}
	teamB := domain.DotaTeam{Name: "Dire"}
	if g.RadiantTeam != nil {
		teamA = *g.RadiantTeam
	}
	if g.DireTeam != nil {
		teamB = *g.DireTeam
	}
	detail := &domain.DotaMatchDetail{
		Match: domain.DotaMatch{
			ID:          matchID,
			TeamA:       teamA,
			TeamB:       teamB,
			ScheduledAt: g.StartedAt,
			Status:      domain.DotaMatchCompleted,
			BestOf:      &bo,
			ScoreA:      &sa,
			ScoreB:      &sb,
		},
		Games: []domain.DotaGame{*g},
		Live:  false,
	}
	s.Sports.writeCache(ctx, key, detail)
	return detail, nil
}

func (s *Service) sportsDotaGameGetOD(ctx context.Context, matchID int, gameIndex int, steamMatchID int64) (*domain.DotaGame, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return nil, domain.ErrInvalidParams
	}
	sid := steamMatchID
	if sid <= 0 {
		detail, err := s.sportsDotaMatchGetOD(ctx, matchID)
		if err != nil {
			return nil, err
		}
		var base *domain.DotaGame
		for i := range detail.Games {
			if detail.Games[i].GameIndex == gameIndex || detail.Games[i].ID == strconv.Itoa(gameIndex) {
				g := detail.Games[i]
				base = &g
				break
			}
		}
		if base == nil && gameIndex > 0 && gameIndex <= len(detail.Games) {
			g := detail.Games[gameIndex-1]
			base = &g
		}
		if base == nil {
			return &domain.DotaGame{
				ID:              fmt.Sprintf("%d:%d", matchID, gameIndex),
				MatchID:         matchID,
				GameIndex:       gameIndex,
				DetailAvailable: false,
				DetailError:     "Game not found for this series",
			}, nil
		}
		if base.StratzMatchID != nil && *base.StratzMatchID > 0 {
			sid = *base.StratzMatchID
		} else if id, err := strconv.ParseInt(base.ID, 10, 64); err == nil && id > 0 {
			sid = id
		} else {
			base.DetailAvailable = false
			base.DetailError = "Steam match id unknown"
			return base, nil
		}
	}
	key := odGameKey(sid)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaDone, "dota.od.game",
		func(g *domain.DotaGame) map[string]any {
			return map[string]any{"steamMatchId": sid, "game": g}
		},
		func(c context.Context) (*domain.DotaGame, error) {
			g, err := s.Sports.OpenDota.GetMatchDetail(c, sid)
			if err != nil {
				return nil, err
			}
			g.MatchID = matchID
			g.GameIndex = gameIndex
			return g, nil
		},
	)
	return out, err
}

func (s *Service) sportsDotaTeamMatchesOD(ctx context.Context, teamID, year int) ([]domain.DotaMatch, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return []domain.DotaMatch{}, nil
	}
	if teamID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := odTeamMatchesKey(teamID, year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaUpcoming, "dota.od.team.matches",
		func(ms []domain.DotaMatch) map[string]any {
			return map[string]any{"teamId": teamID, "year": year, "matches": ms}
		},
		func(c context.Context) ([]domain.DotaMatch, error) {
			ms, err := s.Sports.OpenDota.ListTeamMatchesForYear(c, teamID, year)
			if err != nil {
				return nil, err
			}
			for _, m := range ms {
				s.Sports.writeCache(c, odSeriesMetaKey(m.ID), odSeriesMeta{EventID: m.EventID, Year: year})
			}
			return ms, nil
		},
	)
	return out, err
}

func (s *Service) sportsDotaTeamSearchOD(ctx context.Context, query string) ([]domain.DotaTeam, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return []domain.DotaTeam{}, nil
	}
	if len(query) < 2 {
		return []domain.DotaTeam{}, nil
	}
	return s.Sports.OpenDota.SearchTeams(ctx, query)
}

func (s *Service) sportsDotaTeamGetOD(ctx context.Context, teamID int) (*domain.DotaTeam, error) {
	if s.Sports == nil || s.Sports.OpenDota == nil {
		return nil, domain.ErrInvalidParams
	}
	key := odTeamKey(teamID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaTeams, "dota.od.team",
		func(t *domain.DotaTeam) map[string]any {
			return map[string]any{"teamId": teamID, "team": t}
		},
		func(c context.Context) (*domain.DotaTeam, error) {
			return s.Sports.OpenDota.GetTeam(c, teamID)
		},
	)
	return out, err
}
