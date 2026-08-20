package application

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const (
	ttlDotaEvents   = 12 * time.Hour
	ttlDotaTeams    = 24 * time.Hour
	ttlDotaUpcoming = 3 * time.Minute
	ttlDotaLive     = 12 * time.Second
	ttlDotaDone     = 24 * time.Hour
	ttlDotaStratz   = 24 * time.Hour
)

func dotaEventsKey(year int) string { return fmt.Sprintf("dota.events.%d", year) }
func dotaEventMatchesKey(eventID string) string {
	return fmt.Sprintf("dota.event.matches.%s", eventID)
}
func dotaTeamMatchesKey(teamID, year int) string {
	return fmt.Sprintf("dota.team.matches.%d.%d", teamID, year)
}
func dotaMatchKey(matchID int) string { return fmt.Sprintf("dota.match.%d", matchID) }
func dotaStratzGameKey(steamID int64) string {
	return fmt.Sprintf("dota.stratz.game.%d", steamID)
}

func (s *Service) dotaProvider(ctx context.Context) string {
	if s.Settings != nil {
		if st, err := s.Settings.Get(ctx); err == nil && st != nil && st.DotaProvider == "opendota" {
			return "opendota"
		}
	}
	return "pandascore"
}

func (s *Service) applyDotaProviderTokens(ctx context.Context) {
	if s.Sports == nil {
		return
	}
	pandaTok := ""
	stratzTok := ""
	if s.Settings != nil {
		if st, err := s.Settings.Get(ctx); err == nil && st != nil {
			pandaTok = strings.TrimSpace(st.PandaScoreAPIToken)
			stratzTok = strings.TrimSpace(st.StratzAPIToken)
		}
	}
	// Env fallback for local/dev if settings are empty.
	if pandaTok == "" {
		pandaTok = strings.TrimSpace(os.Getenv("PANDASCORE_API_TOKEN"))
		if pandaTok == "" {
			pandaTok = strings.TrimSpace(os.Getenv("PANDASCORE_TOKEN"))
		}
	}
	if stratzTok == "" {
		stratzTok = strings.TrimSpace(os.Getenv("STRATZ_API_TOKEN"))
		if stratzTok == "" {
			stratzTok = strings.TrimSpace(os.Getenv("STRATZ_TOKEN"))
		}
	}
	if s.Sports.Panda != nil {
		s.Sports.Panda.Token = pandaTok
	}
	if s.Sports.Stratz != nil {
		s.Sports.Stratz.Token = stratzTok
	}
}

func (ss *SportsService) dotaConfigured() domain.DotaProvidersStatus {
	st := domain.DotaProvidersStatus{}
	if ss.Panda != nil {
		st.PandaScoreConfigured = ss.Panda.Configured()
	}
	if ss.Stratz != nil {
		st.StratzConfigured = ss.Stratz.Configured()
	}
	return st
}

func (s *Service) SportsDotaStatus(ctx context.Context) (domain.DotaProvidersStatus, error) {
	if s.Sports == nil {
		return domain.DotaProvidersStatus{}, nil
	}
	s.applyDotaProviderTokens(ctx)
	provider := s.dotaProvider(ctx)
	if provider == "opendota" {
		return s.sportsDotaStatusOD(ctx)
	}
	st := s.Sports.dotaConfigured()
	st.Provider = "pandascore"
	st.Ready = st.PandaScoreConfigured
	return st, nil
}

func (s *Service) SportsDotaYears(ctx context.Context) ([]domain.DotaSeason, error) {
	y := time.Now().UTC().Year()
	out := make([]domain.DotaSeason, 0, 8)
	for i := 0; i < 8; i++ {
		out = append(out, domain.DotaSeason{Year: y - i})
	}
	return out, nil
}

func (s *Service) SportsDotaEvents(ctx context.Context, year int) ([]domain.DotaEvent, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaEventsOD(ctx, year)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return []domain.DotaEvent{}, nil
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := dotaEventsKey(year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaEvents, "dota.events",
		func(ev []domain.DotaEvent) map[string]any {
			return map[string]any{"year": year, "events": ev}
		},
		func(c context.Context) ([]domain.DotaEvent, error) {
			return s.Sports.Panda.ListSeriesForYear(c, year)
		},
	)
	return out, err
}

func (s *Service) SportsDotaEventMatches(ctx context.Context, eventID string) ([]domain.DotaMatch, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaEventMatchesOD(ctx, eventID)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return []domain.DotaMatch{}, nil
	}
	serieID, err := strconv.Atoi(eventID)
	if err != nil || serieID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	key := dotaEventMatchesKey(eventID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaUpcoming, "dota.event.matches",
		func(ms []domain.DotaMatch) map[string]any {
			return map[string]any{"eventId": eventID, "matches": ms}
		},
		func(c context.Context) ([]domain.DotaMatch, error) {
			return s.Sports.Panda.ListSerieMatches(c, serieID)
		},
	)
	return out, err
}

func (s *Service) SportsDotaMatchGet(ctx context.Context, matchID int) (*domain.DotaMatchDetail, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaMatchGetOD(ctx, matchID)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return nil, domain.ErrInvalidParams
	}
	key := dotaMatchKey(matchID)
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
		s.Sports.queueRefresh(key, func(c context.Context) error {
			d, err := s.Sports.Panda.GetMatch(c, matchID)
			if err != nil {
				return err
			}
			s.Sports.writeCache(c, key, d)
			s.Sports.emitCacheUpdated("dota.match", key, map[string]any{"matchId": matchID, "detail": d})
			return nil
		})
		return &cached, nil
	}
	d, err := s.Sports.Panda.GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	s.Sports.writeCache(ctx, key, d)
	return d, nil
}

func (s *Service) SportsDotaGameGet(ctx context.Context, matchID int, gameIndex int, stratzMatchID int64) (*domain.DotaGame, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaGameGetOD(ctx, matchID, gameIndex, stratzMatchID)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil {
		return nil, domain.ErrInvalidParams
	}
	// Prefer explicit STRATZ id when provided.
	if stratzMatchID > 0 && s.Sports.Stratz != nil && s.Sports.Stratz.Configured() {
		return s.fetchStratzGame(ctx, stratzMatchID, matchID, gameIndex)
	}
	detail, err := s.SportsDotaMatchGet(ctx, matchID)
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
			ID:                fmt.Sprintf("%d:%d", matchID, gameIndex),
			MatchID:           matchID,
			GameIndex:         gameIndex,
			DetailAvailable:   false,
			DetailError:       "Game not found for this series",
			MappingConfidence: "unknown",
		}, nil
	}
	if base.StratzMatchID != nil && *base.StratzMatchID > 0 && s.Sports.Stratz != nil && s.Sports.Stratz.Configured() {
		enriched, err := s.fetchStratzGame(ctx, *base.StratzMatchID, matchID, base.GameIndex)
		if err == nil {
			return enriched, nil
		}
		base.DetailError = err.Error()
		base.DetailAvailable = false
		return base, nil
	}

	// PandaScore does not expose Steam match ids — infer via OpenDota pro matches, then load STRATZ.
	steamID, conf := s.resolveSteamMatchID(ctx, detail, base)
	if steamID > 0 {
		base.StratzMatchID = &steamID
		base.MappingConfidence = conf
		if s.Sports.Stratz != nil && s.Sports.Stratz.Configured() {
			enriched, err := s.fetchStratzGame(ctx, steamID, matchID, base.GameIndex)
			if err == nil {
				enriched.MappingConfidence = conf
				return enriched, nil
			}
			base.DetailError = err.Error()
			base.DetailAvailable = false
			return base, nil
		}
		base.DetailAvailable = false
		base.DetailError = "STRATZ token not configured — Steam match found but stats unavailable"
		return base, nil
	}

	base.DetailAvailable = false
	base.MappingConfidence = "unknown"
	if base.DurationSeconds == nil || *base.DurationSeconds <= 0 {
		if detail.Match.Status == domain.DotaMatchCanceled {
			base.DetailError = "This match was canceled — no games were played"
		} else if detail.Match.Status == domain.DotaMatchUpcoming {
			base.DetailError = "Game has not started yet — detailed stats unavailable"
		} else {
			base.DetailError = "Game not finished yet — detailed stats unavailable"
		}
	} else {
		base.DetailError = "Could not map this game to a Steam match — detailed stats unavailable"
	}
	return base, nil
}

func dotaMapKey(psGameID string) string {
	return fmt.Sprintf("dota.map.psgame.%s", psGameID)
}

func (s *Service) resolveSteamMatchID(ctx context.Context, detail *domain.DotaMatchDetail, game *domain.DotaGame) (int64, string) {
	if s.Sports == nil || game == nil {
		return 0, ""
	}
	// Cached mapping from a prior successful resolve.
	if game.ID != "" {
		var cached struct {
			SteamID    int64  `json:"steamId"`
			Confidence string `json:"confidence"`
		}
		if _, ok := s.Sports.readCache(ctx, dotaMapKey(game.ID), &cached); ok && cached.SteamID > 0 {
			return cached.SteamID, cached.Confidence
		}
	}
	if s.Sports.OpenDota == nil || detail == nil {
		return 0, ""
	}
	var startUnix int64
	if game.StartedAt != nil && *game.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, *game.StartedAt); err == nil {
			startUnix = t.Unix()
		}
	}
	dur := 0
	if game.DurationSeconds != nil {
		dur = *game.DurationSeconds
	}
	if startUnix == 0 && dur == 0 {
		return 0, ""
	}
	steamID, conf, ok := s.Sports.OpenDota.FindProMatch(
		ctx,
		startUnix,
		dur,
		detail.Match.TeamA.Name,
		detail.Match.TeamA.ShortName,
		detail.Match.TeamB.Name,
		detail.Match.TeamB.ShortName,
	)
	if !ok || steamID <= 0 {
		return 0, ""
	}
	if conf == "" {
		conf = "inferred"
	}
	if game.ID != "" {
		s.Sports.writeCache(ctx, dotaMapKey(game.ID), map[string]any{
			"steamId":    steamID,
			"confidence": conf,
		})
	}
	return steamID, conf
}

func (s *Service) fetchStratzGame(ctx context.Context, steamID int64, matchID, gameIndex int) (*domain.DotaGame, error) {
	key := dotaStratzGameKey(steamID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaStratz, "dota.stratz.game",
		func(g *domain.DotaGame) map[string]any {
			return map[string]any{"stratzMatchId": steamID, "game": g}
		},
		func(c context.Context) (*domain.DotaGame, error) {
			g, err := s.Sports.Stratz.GetMatchDetail(c, steamID)
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

func (s *Service) SportsDotaTeamMatches(ctx context.Context, teamID, year int) ([]domain.DotaMatch, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaTeamMatchesOD(ctx, teamID, year)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return []domain.DotaMatch{}, nil
	}
	if teamID <= 0 {
		return nil, domain.ErrInvalidParams
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := dotaTeamMatchesKey(teamID, year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaUpcoming, "dota.team.matches",
		func(ms []domain.DotaMatch) map[string]any {
			return map[string]any{"teamId": teamID, "year": year, "matches": ms}
		},
		func(c context.Context) ([]domain.DotaMatch, error) {
			return s.Sports.Panda.ListTeamMatches(c, teamID, year)
		},
	)
	return out, err
}

func (s *Service) SportsDotaTeamSearch(ctx context.Context, query string) ([]domain.DotaTeam, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaTeamSearchOD(ctx, query)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return []domain.DotaTeam{}, nil
	}
	if len(query) < 2 {
		return []domain.DotaTeam{}, nil
	}
	return s.Sports.Panda.SearchTeams(ctx, query)
}

func (s *Service) SportsDotaTeamGet(ctx context.Context, teamID int) (*domain.DotaTeam, error) {
	if s.dotaProvider(ctx) == "opendota" {
		return s.sportsDotaTeamGetOD(ctx, teamID)
	}
	s.applyDotaProviderTokens(ctx)
	if s.Sports == nil || s.Sports.Panda == nil || !s.Sports.Panda.Configured() {
		return nil, domain.ErrInvalidParams
	}
	key := fmt.Sprintf("dota.team.%d", teamID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlDotaTeams, "dota.team",
		func(t *domain.DotaTeam) map[string]any {
			return map[string]any{"teamId": teamID, "team": t}
		},
		func(c context.Context) (*domain.DotaTeam, error) {
			return s.Sports.Panda.GetTeam(c, teamID)
		},
	)
	return out, err
}

func (s *Service) SportsDotaFollowedGet(ctx context.Context) ([]int, error) {
	if s.Sports == nil {
		return []int{}, nil
	}
	ids, err := s.Sports.Repo.GetDotaFollowedTeamIDs(ctx, s.dotaProvider(ctx))
	if err != nil {
		return nil, err
	}
	if ids == nil {
		return []int{}, nil
	}
	return ids, nil
}

func (s *Service) SportsDotaFollowedSet(ctx context.Context, ids []int) ([]int, error) {
	if s.Sports == nil {
		return nil, domain.ErrInvalidParams
	}
	if err := s.Sports.Repo.SetDotaFollowedTeamIDs(ctx, s.dotaProvider(ctx), ids); err != nil {
		return nil, err
	}
	return s.SportsDotaFollowedGet(ctx)
}

func (s *Service) SportsDotaFollowedToggle(ctx context.Context, teamID int) ([]int, error) {
	ids, err := s.SportsDotaFollowedGet(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	next := make([]int, 0, len(ids)+1)
	for _, id := range ids {
		if id == teamID {
			found = true
			continue
		}
		next = append(next, id)
	}
	if !found {
		next = append(next, teamID)
	}
	return s.SportsDotaFollowedSet(ctx, next)
}

func (s *Service) SportsDotaPinnedGet(ctx context.Context) ([]domain.DotaPinnedEvent, error) {
	if s.Sports == nil {
		return []domain.DotaPinnedEvent{}, nil
	}
	pins, err := s.Sports.Repo.GetDotaPinnedEvents(ctx, s.dotaProvider(ctx))
	if err != nil {
		return nil, err
	}
	if pins == nil {
		return []domain.DotaPinnedEvent{}, nil
	}
	return pins, nil
}

func (s *Service) SportsDotaPinnedSet(ctx context.Context, pins []domain.DotaPinnedEvent) ([]domain.DotaPinnedEvent, error) {
	if s.Sports == nil {
		return nil, domain.ErrInvalidParams
	}
	if err := s.Sports.Repo.SetDotaPinnedEvents(ctx, s.dotaProvider(ctx), pins); err != nil {
		return nil, err
	}
	return s.SportsDotaPinnedGet(ctx)
}

func (s *Service) SportsDotaPinnedToggle(ctx context.Context, eventID string, eventType domain.DotaEventType) ([]domain.DotaPinnedEvent, error) {
	pins, err := s.SportsDotaPinnedGet(ctx)
	if err != nil {
		return nil, err
	}
	if eventType == "" {
		eventType = domain.DotaEventTournament
	}
	found := false
	next := make([]domain.DotaPinnedEvent, 0, len(pins)+1)
	for _, p := range pins {
		if p.EventID == eventID && p.EventType == eventType {
			found = true
			continue
		}
		next = append(next, p)
	}
	if !found {
		next = append(next, domain.DotaPinnedEvent{EventID: eventID, EventType: eventType})
	}
	return s.SportsDotaPinnedSet(ctx, next)
}

func (s *Service) SportsDotaMatchWatch(ctx context.Context, matchID int) (*domain.DotaMatchDetail, error) {
	s.applyDotaProviderTokens(ctx)
	d, err := s.SportsDotaMatchGet(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if s.Sports != nil && d.Match.Status == domain.DotaMatchLive {
		s.Sports.startDotaWatch(matchID)
	}
	return d, nil
}

func (s *Service) SportsDotaMatchUnwatch(ctx context.Context, matchID int) error {
	if s.Sports == nil {
		return nil
	}
	s.Sports.stopDotaWatch(matchID)
	return nil
}

func (ss *SportsService) startDotaWatch(matchID int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.dotaWatching == nil {
		ss.dotaWatching = map[int]context.CancelFunc{}
	}
	if _, ok := ss.dotaWatching[matchID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss.dotaWatching[matchID] = cancel
	go ss.dotaPollLoop(ctx, matchID)
}

func (ss *SportsService) stopDotaWatch(matchID int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.dotaWatching == nil {
		return
	}
	if cancel, ok := ss.dotaWatching[matchID]; ok {
		cancel()
		delete(ss.dotaWatching, matchID)
	}
}

func (ss *SportsService) dotaPollLoop(ctx context.Context, matchID int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	defer ss.stopDotaWatch(matchID)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ss.Panda == nil || !ss.Panda.Configured() {
				return
			}
			d, err := ss.Panda.GetMatch(ctx, matchID)
			if err != nil {
				continue
			}
			key := dotaMatchKey(matchID)
			ss.writeCache(ctx, key, d)
			if ss.Emit != nil {
				ss.Emit("sports.dota.match.updated", map[string]any{"matchId": matchID, "detail": d})
			}
			if d.Match.Status != domain.DotaMatchLive {
				return
			}
		}
	}
}
