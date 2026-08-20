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
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type ProMatch struct {
	MatchID     int64  `json:"match_id"`
	StartTime   int64  `json:"start_time"`
	Duration    int    `json:"duration"`
	RadiantName string `json:"radiant_name"`
	DireName    string `json:"dire_name"`
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

	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/proMatches", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; OpenDota)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("opendota HTTP %d: %s", res.StatusCode, truncate(string(body), 120))
	}
	var list []ProMatch
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	c.proMu.Lock()
	c.proCache = list
	c.proAt = time.Now()
	c.proMu.Unlock()
	return list, nil
}

// FindProMatch resolves a Steam match id from start time, duration, and the two team names.
// Returns (steamMatchID, confidence "inferred"|"", ok).
func (c *Client) FindProMatch(
	ctx context.Context,
	startUnix int64,
	durationSec int,
	teamAName, teamAShort, teamBName, teamBShort string,
) (steamID int64, confidence string, ok bool) {
	list, err := c.ListProMatches(ctx)
	if err != nil || len(list) == 0 {
		return 0, "", false
	}
	return FindInProMatches(list, startUnix, durationSec, teamAName, teamAShort, teamBName, teamBShort)
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
