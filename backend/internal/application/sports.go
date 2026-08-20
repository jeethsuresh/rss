package application

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/mlb"
	"github.com/jeeth/rss-reader/backend/internal/openf1"
)

type SportsService struct {
	Repo      domain.SportsRepository
	Client    *mlb.Client
	F1Client  *openf1.Client
	Emit      EventEmitter

	mu         sync.Mutex
	watching   map[int]context.CancelFunc
	f1Watching map[int]context.CancelFunc
}

func NewSportsService(repo domain.SportsRepository, mlbClient *mlb.Client, f1Client *openf1.Client) *SportsService {
	return &SportsService{
		Repo:       repo,
		Client:     mlbClient,
		F1Client:   f1Client,
		watching:   map[int]context.CancelFunc{},
		f1Watching: map[int]context.CancelFunc{},
	}
}

func (s *Service) SportsTeams(ctx context.Context) ([]domain.MlbTeam, error) {
	if s.Sports == nil {
		return []domain.MlbTeam{}, nil
	}
	return s.Sports.Client.ListTeams(ctx)
}

func (s *Service) SportsSeasons(ctx context.Context) ([]domain.MlbSeason, error) {
	if s.Sports == nil {
		return []domain.MlbSeason{}, nil
	}
	return s.Sports.Client.ListSeasons(ctx)
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
		seasons, err := s.Sports.Client.ListSeasons(ctx)
		if err != nil || len(seasons) == 0 {
			season = time.Now().Year()
		} else {
			season = seasons[0].SeasonID
		}
	}
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
	return s.Sports.Client.GameDetail(ctx, gamePk)
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
		seasons, err := s.Sports.Client.ListSeasons(ctx)
		if err != nil || len(seasons) == 0 {
			season = time.Now().Year()
		} else {
			season = seasons[0].SeasonID
		}
	}
	return s.Sports.Client.Standings(ctx, season)
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
	return s.Sports.F1Client.ListYears(ctx)
}

func (s *Service) SportsF1Races(ctx context.Context, year int) ([]domain.F1Race, error) {
	if s.Sports == nil || s.Sports.F1Client == nil {
		return []domain.F1Race{}, nil
	}
	return s.Sports.F1Client.ListRaces(ctx, year)
}

func (s *Service) SportsF1RaceGet(ctx context.Context, sessionKey int) (*domain.F1RaceDetail, error) {
	if s.Sports == nil || s.Sports.F1Client == nil || sessionKey <= 0 {
		return nil, domain.ErrInvalidParams
	}
	return s.Sports.F1Client.RaceDetail(ctx, sessionKey)
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
	return s.Sports.F1Client.Standings(ctx, year)
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
			if ss.Emit != nil {
				ss.Emit("sports.f1.race.updated", detail)
			}
			if detail.Race.Status == domain.F1Completed || detail.Race.Status == domain.F1Cancelled {
				return
			}
		}
	}
}
