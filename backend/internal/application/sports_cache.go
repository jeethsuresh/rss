package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const (
	ttlMeta      = 12 * time.Hour
	ttlSchedule  = 3 * time.Minute
	ttlStandings = 5 * time.Minute
	ttlRoster    = 30 * time.Minute
	ttlRaceList  = 3 * time.Minute
	ttlRaceDone  = 2 * time.Hour
)

func (ss *SportsService) emitRefresh(key, phase string, errMsg string) {
	if ss.Emit == nil {
		return
	}
	payload := map[string]any{"key": key, "phase": phase}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	ss.Emit("sports.refresh", payload)
}

func (ss *SportsService) emitCacheUpdated(resource, key string, extra map[string]any) {
	if ss.Emit == nil {
		return
	}
	payload := map[string]any{"resource": resource, "key": key}
	for k, v := range extra {
		payload[k] = v
	}
	ss.Emit("sports.cache.updated", payload)
}

func (ss *SportsService) readCache(ctx context.Context, key string, dest any) (fetchedAt time.Time, ok bool) {
	if ss.Cache == nil {
		return time.Time{}, false
	}
	raw, at, hit, err := ss.Cache.Get(ctx, key)
	if err != nil || !hit {
		return time.Time{}, false
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return time.Time{}, false
	}
	return at, true
}

func (ss *SportsService) writeCache(ctx context.Context, key string, value any) {
	if ss.Cache == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = ss.Cache.Set(ctx, key, raw)
}

func (ss *SportsService) queueRefresh(key string, run func(context.Context) error) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	started := ss.tryStartRefresh(ctx, key)
	cancel()
	if !started {
		return false
	}
	ss.emitRefresh(key, "started", "")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		err := run(ctx)
		ss.finishRefresh(context.Background(), key)
		if err != nil {
			ss.emitRefresh(key, "error", err.Error())
			return
		}
		ss.emitRefresh(key, "finished", "")
	}()
	return true
}

func (ss *SportsService) tryStartRefresh(ctx context.Context, key string) bool {
	ss.refreshMu.Lock()
	if ss.refreshing == nil {
		ss.refreshing = map[string]bool{}
	}
	if ss.refreshing[key] {
		ss.refreshMu.Unlock()
		return false
	}
	ss.refreshing[key] = true
	ss.refreshMu.Unlock()
	if leases, ok := ss.Cache.(domain.SharedFetchLeaseRepository); ok {
		claimed, err := leases.TryClaim(ctx, "sports:"+key, ss.cacheOwner, 2*time.Minute)
		if err != nil || !claimed {
			ss.refreshMu.Lock()
			delete(ss.refreshing, key)
			ss.refreshMu.Unlock()
			return false
		}
	}
	return true
}

func (ss *SportsService) finishRefresh(ctx context.Context, key string) {
	if leases, ok := ss.Cache.(domain.SharedFetchLeaseRepository); ok {
		_ = leases.Release(ctx, "sports:"+key, ss.cacheOwner)
	}
	ss.refreshMu.Lock()
	delete(ss.refreshing, key)
	ss.refreshMu.Unlock()
}

// getOrFetch returns cached value if present. If missing, fetches synchronously.
// If present but stale, returns cache and queues a background refresh.
func getOrFetch[T any](
	ss *SportsService,
	ctx context.Context,
	key string,
	ttl time.Duration,
	resource string,
	extra func(T) map[string]any,
	fetch func(context.Context) (T, error),
) (T, bool, error) {
	var zero T
	var cached T
	at, ok := ss.readCache(ctx, key, &cached)
	if ok {
		stale := ttl > 0 && time.Since(at) > ttl
		if stale {
			ss.queueRefresh(key, func(rctx context.Context) error {
				fresh, err := fetch(rctx)
				if err != nil {
					return err
				}
				ss.writeCache(rctx, key, fresh)
				payload := map[string]any{"data": fresh}
				if extra != nil {
					for k, v := range extra(fresh) {
						payload[k] = v
					}
				}
				ss.emitCacheUpdated(resource, key, payload)
				return nil
			})
		}
		return cached, true, nil
	}
	for {
		if ss.tryStartRefresh(ctx, key) {
			fresh, err := fetch(ctx)
			if err == nil {
				ss.writeCache(ctx, key, fresh)
			}
			ss.finishRefresh(context.Background(), key)
			if err != nil {
				return zero, false, err
			}
			return fresh, false, nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, false, ctx.Err()
		case <-timer.C:
		}
		if _, ok := ss.readCache(ctx, key, &cached); ok {
			return cached, true, nil
		}
	}
}

func mlbScheduleKey(season, teamID int) string {
	return fmt.Sprintf("mlb.schedule.%d.%d", season, teamID)
}

func mlbStandingsKey(season int) string      { return fmt.Sprintf("mlb.standings.%d", season) }
func mlbDailyScheduleKey(date string) string { return fmt.Sprintf("mlb.schedule.daily.v2.%s", date) }
func mlbRosterKey(teamID, season int) string { return fmt.Sprintf("mlb.roster.%d.%d", teamID, season) }
func f1RacesKey(year int) string             { return fmt.Sprintf("f1.races.%d", year) }
func f1StandingsKey(year int) string         { return fmt.Sprintf("f1.standings.%d", year) }
func f1RaceKey(sessionKey int) string        { return fmt.Sprintf("f1.race.%d", sessionKey) }
func mlbGameKey(gamePk int) string           { return fmt.Sprintf("mlb.game.v2.%d", gamePk) }
