package stratz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const graphqlURL = "https://api.stratz.com/graphql"

// Stay well under default token limits (20/s, 250/min).
const minGap = 100 * time.Millisecond

type Client struct {
	HTTP  *http.Client
	Token string

	rateMu  sync.Mutex
	lastReq time.Time

	heroMu    sync.Mutex
	heroNames map[int]string
	heroesAt  time.Time

	itemMu    sync.Mutex
	itemNames map[int]string
	itemsAt   time.Time
}

func NewClientFromEnv() *Client {
	tok := strings.TrimSpace(os.Getenv("STRATZ_API_TOKEN"))
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("STRATZ_TOKEN"))
	}
	return &Client{
		HTTP:  &http.Client{Timeout: 45 * time.Second},
		Token: tok,
	}
}

func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.Token) != ""
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

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) query(ctx context.Context, query string, vars map[string]any, dest any) error {
	if !c.Configured() {
		return fmt.Errorf("%w: STRATZ token not configured (STRATZ_API_TOKEN)", domain.ErrInvalidParams)
	}
	payload, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.throttle(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; STRATZ)")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrNetwork, err)
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if err != nil {
			return err
		}
		if res.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("%w: stratz rate limited", domain.ErrNetwork)
			timer := time.NewTimer(time.Duration(attempt+1) * 2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: stratz auth failed (%d)", domain.ErrInvalidParams, res.StatusCode)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("%w: stratz HTTP %d: %s", domain.ErrNetwork, res.StatusCode, string(body[:min(200, len(body))]))
		}
		var wrapped gqlResponse
		if err := json.Unmarshal(body, &wrapped); err != nil {
			return fmt.Errorf("%w: stratz decode: %v", domain.ErrParse, err)
		}
		if len(wrapped.Errors) > 0 {
			return fmt.Errorf("%w: stratz: %s", domain.ErrParse, wrapped.Errors[0].Message)
		}
		if dest != nil {
			if err := json.Unmarshal(wrapped.Data, dest); err != nil {
				return fmt.Errorf("%w: stratz data: %v", domain.ErrParse, err)
			}
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%w: stratz failed", domain.ErrNetwork)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) ensureHeroes(ctx context.Context) error {
	c.heroMu.Lock()
	if c.heroNames != nil && time.Since(c.heroesAt) < 24*time.Hour {
		c.heroMu.Unlock()
		return nil
	}
	c.heroMu.Unlock()

	var data struct {
		Constants struct {
			Heroes []struct {
				ID          int    `json:"id"`
				DisplayName string `json:"displayName"`
				ShortName   string `json:"shortName"`
			} `json:"heroes"`
		} `json:"constants"`
	}
	q := `query { constants { heroes { id displayName shortName } } }`
	if err := c.query(ctx, q, nil, &data); err != nil {
		return err
	}
	m := map[int]string{}
	for _, h := range data.Constants.Heroes {
		name := h.DisplayName
		if name == "" {
			name = h.ShortName
		}
		m[h.ID] = name
	}
	c.heroMu.Lock()
	c.heroNames = m
	c.heroesAt = time.Now()
	c.heroMu.Unlock()
	return nil
}

func (c *Client) heroName(ctx context.Context, id int) string {
	_ = c.ensureHeroes(ctx)
	c.heroMu.Lock()
	defer c.heroMu.Unlock()
	if n, ok := c.heroNames[id]; ok {
		return n
	}
	return "Hero " + strconv.Itoa(id)
}

func (c *Client) GetMatchDetail(ctx context.Context, steamMatchID int64) (*domain.DotaGame, error) {
	var data struct {
		Match *struct {
			ID              int64 `json:"id"`
			DurationSeconds int   `json:"durationSeconds"`
			DidRadiantWin   bool  `json:"didRadiantWin"`
			RadiantKills    int   `json:"radiantKills"`
			DireKills       int   `json:"direKills"`
			RadiantTeam     *struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"radiantTeam"`
			DireTeam *struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"direTeam"`
			Players []struct {
				SteamAccountID      int64 `json:"steamAccountId"`
				HeroID              int   `json:"heroId"`
				IsRadiant           bool  `json:"isRadiant"`
				Kills               int   `json:"kills"`
				Deaths              int   `json:"deaths"`
				Assists             int   `json:"assists"`
				GoldPerMinute       int   `json:"goldPerMinute"`
				ExperiencePerMinute int   `json:"experiencePerMinute"`
				Networth            int   `json:"networth"`
				Item0ID             *int  `json:"item0Id"`
				Item1ID             *int  `json:"item1Id"`
				Item2ID             *int  `json:"item2Id"`
				Item3ID             *int  `json:"item3Id"`
				Item4ID             *int  `json:"item4Id"`
				Item5ID             *int  `json:"item5Id"`
				SteamAccount        *struct {
					Name string `json:"name"`
				} `json:"steamAccount"`
			} `json:"players"`
			PickBans []struct {
				HeroID    int  `json:"heroId"`
				IsPick    bool `json:"isPick"`
				IsRadiant bool `json:"isRadiant"`
			} `json:"pickBans"`
		} `json:"match"`
	}
	q := `query ($id: Long!) {
  match(id: $id) {
    id
    durationSeconds
    didRadiantWin
    radiantKills
    direKills
    radiantTeam { id name }
    direTeam { id name }
    players {
      steamAccountId
      heroId
      isRadiant
      kills
      deaths
      assists
      goldPerMinute
      experiencePerMinute
      networth
      item0Id
      item1Id
      item2Id
      item3Id
      item4Id
      item5Id
      steamAccount { name }
    }
    pickBans { heroId isPick isRadiant }
  }
}`
	if err := c.query(ctx, q, map[string]any{"id": steamMatchID}, &data); err != nil {
		return nil, err
	}
	if data.Match == nil {
		return nil, domain.ErrNotFound
	}
	m := data.Match
	_ = c.ensureHeroes(ctx)

	dur := m.DurationSeconds
	rk, dk := m.RadiantKills, m.DireKills
	g := &domain.DotaGame{
		ID:              strconv.FormatInt(m.ID, 10),
		DurationSeconds: &dur,
		RadiantScore:    &rk,
		DireScore:       &dk,
		StratzMatchID:   &steamMatchID,
		MappingConfidence: "exact",
		DetailAvailable: true,
	}
	if m.DidRadiantWin {
		g.Winner = domain.DotaRadiant
	} else {
		g.Winner = domain.DotaDire
	}
	if m.RadiantTeam != nil {
		t := domain.DotaTeam{ID: m.RadiantTeam.ID, Name: m.RadiantTeam.Name}
		g.RadiantTeam = &t
	}
	if m.DireTeam != nil {
		t := domain.DotaTeam{ID: m.DireTeam.ID, Name: m.DireTeam.Name}
		g.DireTeam = &t
	}

	for _, pb := range m.PickBans {
		side := domain.DotaDire
		if pb.IsRadiant {
			side = domain.DotaRadiant
		}
		name := c.heroName(ctx, pb.HeroID)
		if pb.IsPick {
			pid := int64(0)
			g.Heroes = append(g.Heroes, domain.DotaHeroPick{
				HeroID: pb.HeroID, HeroName: name, Team: side, PlayerID: nil,
			})
			_ = pid
		} else {
			g.Bans = append(g.Bans, domain.DotaHeroBan{HeroID: pb.HeroID, HeroName: name, Team: side})
		}
	}

	// Prefer player roster for picks if pickBans empty
	if len(g.Heroes) == 0 {
		for _, p := range m.Players {
			side := domain.DotaDire
			if p.IsRadiant {
				side = domain.DotaRadiant
			}
			pid := p.SteamAccountID
			g.Heroes = append(g.Heroes, domain.DotaHeroPick{
				HeroID: p.HeroID, HeroName: c.heroName(ctx, p.HeroID), PlayerID: &pid, Team: side,
			})
		}
	}

	for _, p := range m.Players {
		side := domain.DotaDire
		if p.IsRadiant {
			side = domain.DotaRadiant
		}
		name := ""
		if p.SteamAccount != nil {
			name = p.SteamAccount.Name
		}
		if name == "" {
			name = "Player"
		}
		items := []domain.DotaItem{}
		for _, idp := range []*int{p.Item0ID, p.Item1ID, p.Item2ID, p.Item3ID, p.Item4ID, p.Item5ID} {
			if idp == nil || *idp <= 0 {
				continue
			}
			items = append(items, domain.DotaItem{ItemID: *idp})
		}
		g.Players = append(g.Players, domain.DotaPlayer{
			PlayerID: p.SteamAccountID,
			Name:     name,
			HeroID:   p.HeroID,
			HeroName: c.heroName(ctx, p.HeroID),
			Team:     side,
			Kills:    p.Kills,
			Deaths:   p.Deaths,
			Assists:  p.Assists,
			GPM:      p.GoldPerMinute,
			XPM:      p.ExperiencePerMinute,
			NetWorth: p.Networth,
			Items:    items,
		})
	}
	return g, nil
}
