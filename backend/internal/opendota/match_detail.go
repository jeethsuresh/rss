package opendota

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type odMatchDetail struct {
	MatchID      int64 `json:"match_id"`
	Duration     int   `json:"duration"`
	StartTime    int64 `json:"start_time"`
	RadiantWin   bool  `json:"radiant_win"`
	RadiantScore int   `json:"radiant_score"`
	DireScore    int   `json:"dire_score"`
	RadiantTeam  *struct {
		TeamID int    `json:"team_id"`
		Name   string `json:"name"`
	} `json:"radiant_team"`
	DireTeam *struct {
		TeamID int    `json:"team_id"`
		Name   string `json:"name"`
	} `json:"dire_team"`
	Players []struct {
		AccountID   int64  `json:"account_id"`
		HeroID      int    `json:"hero_id"`
		IsRadiant   *bool  `json:"isRadiant"`
		PlayerSlot  int    `json:"player_slot"`
		Personaname string `json:"personaname"`
		Name        string `json:"name"`
		Kills       int    `json:"kills"`
		Deaths      int    `json:"deaths"`
		Assists     int    `json:"assists"`
		GoldPerMin  int    `json:"gold_per_min"`
		XpPerMin    int    `json:"xp_per_min"`
		NetWorth    int    `json:"net_worth"`
		Item0       int    `json:"item_0"`
		Item1       int    `json:"item_1"`
		Item2       int    `json:"item_2"`
		Item3       int    `json:"item_3"`
		Item4       int    `json:"item_4"`
		Item5       int    `json:"item_5"`
	} `json:"players"`
	PicksBans []struct {
		IsPick bool `json:"is_pick"`
		HeroID int  `json:"hero_id"`
		Team   int  `json:"team"` // 0 radiant, 1 dire
		Order  int  `json:"order"`
	} `json:"picks_bans"`
}

func (c *Client) ensureHeroNames(ctx context.Context) error {
	c.heroMu.Lock()
	if c.heroNames != nil && time.Since(c.heroesAt) < 24*time.Hour {
		c.heroMu.Unlock()
		return nil
	}
	c.heroMu.Unlock()

	var raw map[string]struct {
		ID            int    `json:"id"`
		LocalizedName string `json:"localized_name"`
	}
	if err := c.getJSON(ctx, "/constants/heroes", &raw); err != nil {
		return err
	}
	m := map[int]string{}
	for _, h := range raw {
		if h.ID > 0 && h.LocalizedName != "" {
			m[h.ID] = h.LocalizedName
		}
	}
	c.heroMu.Lock()
	c.heroNames = m
	c.heroesAt = time.Now()
	c.heroMu.Unlock()
	return nil
}

func (c *Client) heroName(ctx context.Context, id int) string {
	_ = c.ensureHeroNames(ctx)
	c.heroMu.Lock()
	defer c.heroMu.Unlock()
	if n, ok := c.heroNames[id]; ok {
		return n
	}
	return fmt.Sprintf("Hero %d", id)
}

func (c *Client) GetMatchDetail(ctx context.Context, steamMatchID int64) (*domain.DotaGame, error) {
	var raw odMatchDetail
	if err := c.getJSON(ctx, fmt.Sprintf("/matches/%d", steamMatchID), &raw); err != nil {
		return nil, err
	}
	if raw.MatchID == 0 {
		return nil, domain.ErrNotFound
	}
	_ = c.ensureHeroNames(ctx)

	dur := raw.Duration
	rk, dk := raw.RadiantScore, raw.DireScore
	sid := steamMatchID
	g := &domain.DotaGame{
		ID:                strconv.FormatInt(raw.MatchID, 10),
		DurationSeconds:   &dur,
		RadiantScore:      &rk,
		DireScore:         &dk,
		StratzMatchID:     &sid,
		MappingConfidence: "exact",
		DetailAvailable:   true,
	}
	if raw.StartTime > 0 {
		st := time.Unix(raw.StartTime, 0).UTC().Format(time.RFC3339)
		g.StartedAt = &st
	}
	if raw.RadiantWin {
		g.Winner = domain.DotaRadiant
	} else {
		g.Winner = domain.DotaDire
	}
	if raw.RadiantTeam != nil {
		t := domain.DotaTeam{ID: raw.RadiantTeam.TeamID, Name: raw.RadiantTeam.Name}
		g.RadiantTeam = &t
		if raw.RadiantWin {
			g.WinnerTeamName = t.Name
		}
	}
	if raw.DireTeam != nil {
		t := domain.DotaTeam{ID: raw.DireTeam.TeamID, Name: raw.DireTeam.Name}
		g.DireTeam = &t
		if !raw.RadiantWin {
			g.WinnerTeamName = t.Name
		}
	}

	for _, pb := range raw.PicksBans {
		side := domain.DotaRadiant
		if pb.Team == 1 {
			side = domain.DotaDire
		}
		name := c.heroName(ctx, pb.HeroID)
		if pb.IsPick {
			g.Heroes = append(g.Heroes, domain.DotaHeroPick{
				HeroID:   pb.HeroID,
				HeroName: name,
				Team:     side,
			})
		} else {
			g.Bans = append(g.Bans, domain.DotaHeroBan{
				HeroID:   pb.HeroID,
				HeroName: name,
				Team:     side,
			})
		}
	}

	for _, p := range raw.Players {
		isRad := p.PlayerSlot < 128
		if p.IsRadiant != nil {
			isRad = *p.IsRadiant
		}
		side := domain.DotaDire
		if isRad {
			side = domain.DotaRadiant
		}
		name := p.Personaname
		if name == "" {
			name = p.Name
		}
		if name == "" {
			name = fmt.Sprintf("Player %d", p.AccountID)
		}
		items := []domain.DotaItem{}
		for _, iid := range []int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5} {
			if iid > 0 {
				items = append(items, domain.DotaItem{ItemID: iid, Name: fmt.Sprintf("Item %d", iid)})
			}
		}
		g.Players = append(g.Players, domain.DotaPlayer{
			PlayerID: p.AccountID,
			Name:     name,
			HeroID:   p.HeroID,
			HeroName: c.heroName(ctx, p.HeroID),
			Team:     side,
			Kills:    p.Kills,
			Deaths:   p.Deaths,
			Assists:  p.Assists,
			GPM:      p.GoldPerMin,
			XPM:      p.XpPerMin,
			NetWorth: p.NetWorth,
			Items:    items,
		})
	}
	return g, nil
}
