package opendota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://api.opendota.com/api"

// Public API — be polite.
const minGap = 200 * time.Millisecond

type Client struct {
	HTTP *http.Client

	rateMu  sync.Mutex
	lastReq time.Time

	proMu    sync.Mutex
	proCache []ProMatch
	proAt    time.Time

	teamMu    sync.Mutex
	teamByKey map[string]int // norm name/tag → team_id
	teamsAt   time.Time
}

func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		teamByKey: map[string]int{},
	}
}

type ProMatch struct {
	MatchID     int64  `json:"match_id"`
	StartTime   int64  `json:"start_time"`
	Duration    int    `json:"duration"`
	RadiantName string `json:"radiant_name"`
	DireName    string `json:"dire_name"`
}

type TeamMatch struct {
	MatchID          int64  `json:"match_id"`
	StartTime        int64  `json:"start_time"`
	Duration         int    `json:"duration"`
	OpposingTeamName string `json:"opposing_team_name"`
}

type odTeam struct {
	TeamID int    `json:"team_id"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; OpenDota)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("opendota HTTP %d: %s", res.StatusCode, truncate(string(body), 120))
	}
	return json.Unmarshal(body, dest)
}

func (c *Client) throttle(ctx context.Context) error {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	wait := minGap - time.Since(c.lastReq)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	c.lastReq = time.Now()
	return nil
}

func (c *Client) ListProMatches(ctx context.Context) ([]ProMatch, error) {
	c.proMu.Lock()
	if len(c.proCache) > 0 && time.Since(c.proAt) < 2*time.Minute {
		out := append([]ProMatch(nil), c.proCache...)
		c.proMu.Unlock()
		return out, nil
	}
	c.proMu.Unlock()

	var list []ProMatch
	if err := c.getJSON(ctx, "/proMatches", &list); err != nil {
		return nil, err
	}
	c.proMu.Lock()
	c.proCache = list
	c.proAt = time.Now()
	c.proMu.Unlock()
	return list, nil
}

func (c *Client) ensureTeams(ctx context.Context) error {
	c.teamMu.Lock()
	if len(c.teamByKey) > 0 && time.Since(c.teamsAt) < 12*time.Hour {
		c.teamMu.Unlock()
		return nil
	}
	c.teamMu.Unlock()

	var list []odTeam
	if err := c.getJSON(ctx, "/teams", &list); err != nil {
		return err
	}
	m := map[string]int{}
	for _, t := range list {
		if t.TeamID <= 0 {
			continue
		}
		if n := norm(t.Name); n != "" {
			if _, ok := m[n]; !ok {
				m[n] = t.TeamID
			}
		}
		if tag := norm(t.Tag); tag != "" && len(tag) >= 2 {
			if _, ok := m[tag]; !ok {
				m[tag] = t.TeamID
			}
		}
	}
	c.teamMu.Lock()
	c.teamByKey = m
	c.teamsAt = time.Now()
	c.teamMu.Unlock()
	return nil
}

func (c *Client) resolveTeamID(ctx context.Context, name, short string) (int, bool) {
	if err := c.ensureTeams(ctx); err != nil {
		return 0, false
	}
	c.teamMu.Lock()
	defer c.teamMu.Unlock()
	for _, key := range []string{norm(name), norm(short)} {
		if key == "" {
			continue
		}
		if id, ok := c.teamByKey[key]; ok {
			return id, true
		}
	}
	// substring fallback
	n := norm(name)
	if n == "" {
		return 0, false
	}
	for k, id := range c.teamByKey {
		if strings.Contains(k, n) || strings.Contains(n, k) {
			return id, true
		}
	}
	return 0, false
}

func (c *Client) ListTeamMatches(ctx context.Context, teamID int) ([]TeamMatch, error) {
	if teamID <= 0 {
		return nil, fmt.Errorf("invalid team id")
	}
	var list []TeamMatch
	if err := c.getJSON(ctx, fmt.Sprintf("/teams/%d/matches", teamID), &list); err != nil {
		return nil, err
	}
	return list, nil
}

// FindProMatch resolves a Steam match id from start time, duration, and the two team names.
// Tries recent /proMatches first, then OpenDota team match history (covers older games).
func (c *Client) FindProMatch(
	ctx context.Context,
	startUnix int64,
	durationSec int,
	teamAName, teamAShort, teamBName, teamBShort string,
) (steamID int64, confidence string, ok bool) {
	if list, err := c.ListProMatches(ctx); err == nil && len(list) > 0 {
		if id, conf, hit := FindInProMatches(list, startUnix, durationSec, teamAName, teamAShort, teamBName, teamBShort); hit {
			return id, conf, true
		}
	}
	return c.findInTeamHistory(ctx, startUnix, durationSec, teamAName, teamAShort, teamBName, teamBShort)
}

func (c *Client) findInTeamHistory(
	ctx context.Context,
	startUnix int64,
	durationSec int,
	teamAName, teamAShort, teamBName, teamBShort string,
) (int64, string, bool) {
	type side struct{ name, short string }
	sides := []side{{teamAName, teamAShort}, {teamBName, teamBShort}}
	for i, self := range sides {
		opp := sides[1-i]
		teamID, ok := c.resolveTeamID(ctx, self.name, self.short)
		if !ok {
			continue
		}
		list, err := c.ListTeamMatches(ctx, teamID)
		if err != nil || len(list) == 0 {
			continue
		}
		if id, hit := FindInTeamMatches(list, startUnix, durationSec, opp.name, opp.short); hit {
			return id, "inferred", true
		}
	}
	return 0, "", false
}

// FindInTeamMatches matches OpenDota /teams/{id}/matches rows.
func FindInTeamMatches(list []TeamMatch, startUnix int64, durationSec int, oppName, oppShort string) (int64, bool) {
	type cand struct {
		id    int64
		score int
	}
	best := cand{}
	for _, m := range list {
		score := 0
		if startUnix > 0 {
			dt := abs64(m.StartTime - startUnix)
			if dt > 20*60 {
				continue
			}
			score += 50 - int(dt/30)
		}
		if durationSec > 0 && m.Duration > 0 {
			dd := absInt(m.Duration - durationSec)
			if dd > 90 {
				continue
			}
			score += 40 - dd/3
		}
		if oppName != "" || oppShort != "" {
			if !teamMatches(m.OpposingTeamName, oppName, oppShort) {
				continue
			}
			score += 30
		}
		if score > best.score {
			best = cand{id: m.MatchID, score: score}
		}
	}
	if best.id == 0 || best.score < 60 {
		return 0, false
	}
	return best.id, true
}

// FindInProMatches is the pure matcher used by FindProMatch (exported for tests).
func FindInProMatches(
	list []ProMatch,
	startUnix int64,
	durationSec int,
	teamAName, teamAShort, teamBName, teamBShort string,
) (steamID int64, confidence string, ok bool) {
	type cand struct {
		id    int64
		score int
	}
	best := cand{}
	for _, m := range list {
		score := 0
		if startUnix > 0 {
			dt := abs64(m.StartTime - startUnix)
			if dt > 15*60 {
				continue
			}
			score += 50 - int(dt/30) // closer start → higher
		}
		if durationSec > 0 && m.Duration > 0 {
			dd := absInt(m.Duration - durationSec)
			if dd > 90 {
				continue
			}
			score += 40 - dd/3
		}
		radOK := teamMatches(m.RadiantName, teamAName, teamAShort) || teamMatches(m.RadiantName, teamBName, teamBShort)
		direOK := teamMatches(m.DireName, teamAName, teamAShort) || teamMatches(m.DireName, teamBName, teamBShort)
		if !(radOK && direOK) {
			if m.RadiantName != "" || m.DireName != "" {
				continue
			}
		} else {
			score += 30
		}
		aOnRad := teamMatches(m.RadiantName, teamAName, teamAShort)
		aOnDire := teamMatches(m.DireName, teamAName, teamAShort)
		bOnRad := teamMatches(m.RadiantName, teamBName, teamBShort)
		bOnDire := teamMatches(m.DireName, teamBName, teamBShort)
		if (aOnRad && bOnDire) || (aOnDire && bOnRad) {
			score += 20
		}
		if score > best.score {
			best = cand{id: m.MatchID, score: score}
		}
	}
	if best.id == 0 || best.score < 60 {
		return 0, "", false
	}
	return best.id, "inferred", true
}

func teamMatches(odName, name, short string) bool {
	od := norm(odName)
	if od == "" {
		return false
	}
	n := norm(name)
	s := norm(short)
	if n != "" && (od == n || strings.Contains(od, n) || strings.Contains(n, od)) {
		return true
	}
	if s != "" && len(s) >= 2 && (od == s || strings.Contains(od, s) || strings.HasPrefix(od, s)) {
		return true
	}
	return false
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(".", "", "-", "", "_", "", "  ", " ")
	return repl.Replace(s)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
