package application

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/mlb"
	"github.com/jeeth/rss-reader/backend/internal/opendota"
	"github.com/jeeth/rss-reader/backend/internal/openf1"
	"github.com/jeeth/rss-reader/backend/internal/pandascore"
	"github.com/jeeth/rss-reader/backend/internal/stratz"
)

type SportsService struct {
	Repo     domain.SportsRepository
	Cache    domain.SportsCacheRepository
	Client   *mlb.Client
	F1Client *openf1.Client
	Panda    *pandascore.Client
	Stratz   *stratz.Client
	OpenDota *opendota.Client
	Emit     EventEmitter

	mu          sync.Mutex
	watching    map[int]context.CancelFunc
	f1Watching  map[int]context.CancelFunc
	dotaWatching map[int]context.CancelFunc

	refreshMu  sync.Mutex
	refreshing map[string]bool
}

func NewSportsService(
	repo domain.SportsRepository,
	cache domain.SportsCacheRepository,
	mlbClient *mlb.Client,
	f1Client *openf1.Client,
	panda *pandascore.Client,
	stratzClient *stratz.Client,
	openDota *opendota.Client,
) *SportsService {
	return &SportsService{
		Repo:         repo,
		Cache:        cache,
		Client:       mlbClient,
		F1Client:     f1Client,
		Panda:        panda,
		Stratz:       stratzClient,
		OpenDota:     openDota,
		watching:     map[int]context.CancelFunc{},
		f1Watching:   map[int]context.CancelFunc{},
		dotaWatching: map[int]context.CancelFunc{},
		refreshing:   map[string]bool{},
	}
}

func (s *Service) SportsTeams(ctx context.Context) ([]domain.MlbTeam, error) {
	if s.Sports == nil {
		return []domain.MlbTeam{}, nil
	}
	out, _, err := getOrFetch(s.Sports, ctx, "mlb.teams", ttlMeta, "mlb.teams", nil, func(c context.Context) ([]domain.MlbTeam, error) {
		return s.Sports.Client.ListTeams(c)
	})
	return out, err
}

func (s *Service) SportsSeasons(ctx context.Context) ([]domain.MlbSeason, error) {
	if s.Sports == nil {
		return []domain.MlbSeason{}, nil
	}
	out, _, err := getOrFetch(s.Sports, ctx, "mlb.seasons", ttlMeta, "mlb.seasons", nil, func(c context.Context) ([]domain.MlbSeason, error) {
		return s.Sports.Client.ListSeasons(c)
	})
	return out, err
}

func (s *Service) SportsFollowedGet(ctx context.Context) ([]int, error) {
	if s.Sports == nil {
		return []int{}, nil
	}
	ids, err := s.Sports.Repo.GetFollowedTeamIDs(ctx)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		return []int{}, nil
	}
	return ids, nil
}

func (s *Service) SportsFollowedSet(ctx context.Context, ids []int) ([]int, error) {
	if s.Sports == nil {
		return nil, domain.ErrInvalidParams
	}
	if err := s.Sports.Repo.SetFollowedTeamIDs(ctx, ids); err != nil {
		return nil, err
	}
	return s.SportsFollowedGet(ctx)
}

func (s *Service) SportsFollowedToggle(ctx context.Context, teamID int) ([]int, error) {
	ids, err := s.SportsFollowedGet(ctx)
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
	return s.SportsFollowedSet(ctx, next)
}

func (s *Service) SportsSchedule(ctx context.Context, teamID, season int) ([]domain.MlbGame, error) {
	if s.Sports == nil {
		return []domain.MlbGame{}, nil
	}
	if season <= 0 {
		seasons, err := s.SportsSeasons(ctx)
		if err != nil || len(seasons) == 0 {
			season = time.Now().Year()
		} else {
			season = seasons[0].SeasonID
		}
	}
	key := mlbScheduleKey(season, teamID)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlSchedule, "mlb.schedule",
		func(games []domain.MlbGame) map[string]any {
			return map[string]any{"season": season, "teamId": teamID, "games": games}
		},
		func(c context.Context) ([]domain.MlbGame, error) {
			return s.fetchMlbSchedule(c, teamID, season)
		},
	)
	return out, err
}

func (s *Service) fetchMlbSchedule(ctx context.Context, teamID, season int) ([]domain.MlbGame, error) {
	var teamIDs []int
	if teamID > 0 {
		teamIDs = []int{teamID}
	} else {
		var err error
		teamIDs, err = s.SportsFollowedGet(ctx)
		if err != nil {
			return nil, err
		}
	}
	byPk := map[int]domain.MlbGame{}
	for _, tid := range teamIDs {
		games, err := s.Sports.Client.TeamSchedule(ctx, tid, season)
		if err != nil {
			return nil, err
		}
		for _, g := range games {
			byPk[g.ID] = g
		}
	}
	out := make([]domain.MlbGame, 0, len(byPk))
	for _, g := range byPk {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GameDate > out[j].GameDate
	})
	return out, nil
}

func (s *Service) SportsGameGet(ctx context.Context, gamePk int) (*domain.MlbGameDetail, error) {
	if s.Sports == nil || gamePk <= 0 {
		return nil, domain.ErrInvalidParams
	}
	key := mlbGameKey(gamePk)
	var cached domain.MlbGameDetail
	if at, ok := s.Sports.readCache(ctx, key, &cached); ok {
		finalish := cached.Game.Status == domain.MlbFinal ||
			cached.Game.Status == domain.MlbCancelled ||
			cached.Game.Status == domain.MlbPostponed
		if finalish {
			return &cached, nil
		}
		// Live / upcoming: serve cache briefly then refresh in background.
		if time.Since(at) < 20*time.Second {
			return &cached, nil
		}
		s.Sports.queueRefresh(key, func(rctx context.Context) error {
			detail, err := s.Sports.Client.GameDetail(rctx, gamePk)
			if err != nil {
				return err
			}
			s.Sports.writeCache(rctx, key, detail)
			s.Sports.emitCacheUpdated("mlb.game", key, map[string]any{"gamePk": gamePk, "detail": detail})
			if s.Sports.Emit != nil {
				s.Sports.Emit("sports.game.updated", detail)
			}
			return nil
		})
		return &cached, nil
	}
	detail, err := s.Sports.Client.GameDetail(ctx, gamePk)
	if err != nil {
		return nil, err
	}
	s.Sports.writeCache(ctx, key, detail)
	return detail, nil
}

func (s *Service) SportsGameWatch(ctx context.Context, gamePk int) (*domain.MlbGameDetail, error) {
	detail, err := s.SportsGameGet(ctx, gamePk)
	if err != nil {
		return nil, err
	}
	if s.Sports == nil {
		return detail, nil
	}
	s.Sports.startWatch(gamePk)
	return detail, nil
}

func (s *Service) SportsGameUnwatch(_ context.Context, gamePk int) error {
	if s.Sports == nil {
		return nil
	}
	s.Sports.stopWatch(gamePk)
	return nil
}

func (s *Service) SportsStandings(ctx context.Context, season int) (*domain.MlbStandings, error) {
	if s.Sports == nil || s.Sports.Client == nil {
		return nil, domain.ErrInvalidParams
	}
	if season <= 0 {
		seasons, err := s.SportsSeasons(ctx)
		if err != nil || len(seasons) == 0 {
			season = time.Now().Year()
		} else {
			season = seasons[0].SeasonID
		}
	}
	key := mlbStandingsKey(season)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlStandings, "mlb.standings",
		func(st domain.MlbStandings) map[string]any {
			return map[string]any{"season": season, "standings": st}
		},
		func(c context.Context) (domain.MlbStandings, error) {
			st, err := s.Sports.Client.Standings(c, season)
			if err != nil {
				return domain.MlbStandings{}, err
			}
			return *st, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (ss *SportsService) startWatch(gamePk int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _, ok := ss.watching[gamePk]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss.watching[gamePk] = cancel
	go ss.pollLoop(ctx, gamePk)
}

func (ss *SportsService) stopWatch(gamePk int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if cancel, ok := ss.watching[gamePk]; ok {
		cancel()
		delete(ss.watching, gamePk)
	}
}

func (ss *SportsService) pollLoop(ctx context.Context, gamePk int) {
	ticker := time.NewTicker(18 * time.Second)
	defer ticker.Stop()
	defer func() {
		ss.mu.Lock()
		delete(ss.watching, gamePk)
		ss.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			detail, err := ss.Client.GameDetail(ctx, gamePk)
			if err != nil {
				continue
			}
			ss.writeCache(ctx, mlbGameKey(gamePk), detail)
			if ss.Emit != nil {
				ss.Emit("sports.game.updated", detail)
			}
			if detail.Game.Status == domain.MlbFinal ||
				detail.Game.Status == domain.MlbCancelled ||
				detail.Game.Status == domain.MlbPostponed {
				return
			}
		}
	}
}

func (s *Service) SportsF1Years(ctx context.Context) ([]domain.F1Season, error) {
	if s.Sports == nil || s.Sports.F1Client == nil {
		return []domain.F1Season{}, nil
	}
	out, _, err := getOrFetch(s.Sports, ctx, "f1.years", ttlMeta, "f1.years", nil, func(c context.Context) ([]domain.F1Season, error) {
		return s.Sports.F1Client.ListYears(c)
	})
	return out, err
}

func (s *Service) SportsF1Races(ctx context.Context, year int) ([]domain.F1Race, error) {
	if s.Sports == nil || s.Sports.F1Client == nil {
		return []domain.F1Race{}, nil
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := f1RacesKey(year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlRaceList, "f1.races",
		func(races []domain.F1Race) map[string]any {
			return map[string]any{"year": year, "races": races}
		},
		func(c context.Context) ([]domain.F1Race, error) {
			return s.Sports.F1Client.ListRaces(c, year)
		},
	)
	return out, err
}

func (s *Service) SportsF1RaceGet(ctx context.Context, sessionKey int) (*domain.F1RaceDetail, error) {
	if s.Sports == nil || s.Sports.F1Client == nil || sessionKey <= 0 {
		return nil, domain.ErrInvalidParams
	}
	key := f1RaceKey(sessionKey)
	var cached domain.F1RaceDetail
	if at, ok := s.Sports.readCache(ctx, key, &cached); ok {
		done := cached.Race.Status == domain.F1Completed || cached.Race.Status == domain.F1Cancelled
		if done || time.Since(at) < 20*time.Second {
			if !done {
				s.Sports.queueRefresh(key, func(rctx context.Context) error {
					detail, err := s.Sports.F1Client.RaceDetail(rctx, sessionKey)
					if err != nil {
						return err
					}
					s.Sports.writeCache(rctx, key, detail)
					s.Sports.emitCacheUpdated("f1.race", key, map[string]any{"sessionKey": sessionKey, "detail": detail})
					if s.Sports.Emit != nil {
						s.Sports.Emit("sports.f1.race.updated", detail)
					}
					return nil
				})
			}
			return &cached, nil
		}
		s.Sports.queueRefresh(key, func(rctx context.Context) error {
			detail, err := s.Sports.F1Client.RaceDetail(rctx, sessionKey)
			if err != nil {
				return err
			}
			s.Sports.writeCache(rctx, key, detail)
			s.Sports.emitCacheUpdated("f1.race", key, map[string]any{"sessionKey": sessionKey, "detail": detail})
			if s.Sports.Emit != nil {
				s.Sports.Emit("sports.f1.race.updated", detail)
			}
			return nil
		})
		return &cached, nil
	}
	detail, err := s.Sports.F1Client.RaceDetail(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	s.Sports.writeCache(ctx, key, detail)
	return detail, nil
}

func (s *Service) SportsF1RaceWatch(ctx context.Context, sessionKey int) (*domain.F1RaceDetail, error) {
	detail, err := s.SportsF1RaceGet(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if s.Sports == nil {
		return detail, nil
	}
	s.Sports.startF1Watch(sessionKey)
	return detail, nil
}

func (s *Service) SportsF1RaceUnwatch(_ context.Context, sessionKey int) error {
	if s.Sports == nil {
		return nil
	}
	s.Sports.stopF1Watch(sessionKey)
	return nil
}

func (s *Service) SportsF1Standings(ctx context.Context, year int) (*domain.F1Standings, error) {
	if s.Sports == nil || s.Sports.F1Client == nil {
		return nil, domain.ErrInvalidParams
	}
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	key := f1StandingsKey(year)
	out, _, err := getOrFetch(s.Sports, ctx, key, ttlStandings, "f1.standings",
		func(st domain.F1Standings) map[string]any {
			return map[string]any{"year": year, "standings": st}
		},
		func(c context.Context) (domain.F1Standings, error) {
			st, err := s.Sports.F1Client.Standings(c, year)
			if err != nil {
				return domain.F1Standings{}, err
			}
			return *st, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (ss *SportsService) startF1Watch(sessionKey int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _, ok := ss.f1Watching[sessionKey]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss.f1Watching[sessionKey] = cancel
	go ss.f1PollLoop(ctx, sessionKey)
}

func (ss *SportsService) stopF1Watch(sessionKey int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if cancel, ok := ss.f1Watching[sessionKey]; ok {
		cancel()
		delete(ss.f1Watching, sessionKey)
	}
}

func (ss *SportsService) f1PollLoop(ctx context.Context, sessionKey int) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	defer func() {
		ss.mu.Lock()
		delete(ss.f1Watching, sessionKey)
		ss.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ss.F1Client == nil {
				return
			}
			detail, err := ss.F1Client.RaceDetail(ctx, sessionKey)
			if err != nil {
				continue
			}
			ss.writeCache(ctx, f1RaceKey(sessionKey), detail)
			if ss.Emit != nil {
				ss.Emit("sports.f1.race.updated", detail)
			}
			if detail.Race.Status == domain.F1Completed || detail.Race.Status == domain.F1Cancelled {
				return
			}
		}
	}
}
