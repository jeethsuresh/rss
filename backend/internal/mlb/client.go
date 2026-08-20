package mlb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const baseURL = "https://statsapi.mlb.com/api"

type Client struct {
	HTTP *http.Client

	mu          sync.Mutex
	teamsCache  []domain.MlbTeam
	teamsAt     time.Time
	seasonsCache []domain.MlbSeason
	seasonsAt   time.Time
}

func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, dest any) error {
	u := baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; MLB stats)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrNetwork, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 12<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: mlb status %d", domain.ErrNetwork, res.StatusCode)
	}
	return json.Unmarshal(body, dest)
}

func logoURL(teamID int) string {
	return fmt.Sprintf("https://www.mlbstatic.com/team-logos/%d.svg", teamID)
}

func (c *Client) ListTeams(ctx context.Context) ([]domain.MlbTeam, error) {
	c.mu.Lock()
	if time.Since(c.teamsAt) < 12*time.Hour && len(c.teamsCache) > 0 {
		out := append([]domain.MlbTeam{}, c.teamsCache...)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	var raw struct {
		Teams []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Abbreviation string `json:"abbreviation"`
			TeamName     string `json:"teamName"`
		} `json:"teams"`
	}
	q := url.Values{}
	q.Set("sportId", "1")
	if err := c.getJSON(ctx, "/v1/teams", q, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.MlbTeam, 0, len(raw.Teams))
	for _, t := range raw.Teams {
		out = append(out, domain.MlbTeam{
			ID: t.ID, Name: t.Name, Abbreviation: t.Abbreviation,
			ShortName: t.TeamName, LogoURL: logoURL(t.ID),
		})
	}
	c.mu.Lock()
	c.teamsCache = out
	c.teamsAt = time.Now()
	c.mu.Unlock()
	return out, nil
}

func (c *Client) ListSeasons(ctx context.Context) ([]domain.MlbSeason, error) {
	c.mu.Lock()
	if time.Since(c.seasonsAt) < 24*time.Hour && len(c.seasonsCache) > 0 {
		out := append([]domain.MlbSeason{}, c.seasonsCache...)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	var raw struct {
		Seasons []struct {
			SeasonID               string `json:"seasonId"`
			RegularSeasonStartDate string `json:"regularSeasonStartDate"`
			RegularSeasonEndDate   string `json:"regularSeasonEndDate"`
		} `json:"seasons"`
	}
	q := url.Values{}
	q.Set("sportId", "1")
	q.Set("all", "true")
	if err := c.getJSON(ctx, "/v1/seasons", q, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.MlbSeason, 0, len(raw.Seasons))
	for _, s := range raw.Seasons {
		id, _ := strconv.Atoi(s.SeasonID)
		if id == 0 {
			continue
		}
		out = append(out, domain.MlbSeason{
			SeasonID: id,
			RegularSeasonStartDate: s.RegularSeasonStartDate,
			RegularSeasonEndDate:   s.RegularSeasonEndDate,
		})
	}
	// newest first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	c.mu.Lock()
	c.seasonsCache = out
	c.seasonsAt = time.Now()
	c.mu.Unlock()
	return out, nil
}

func (c *Client) TeamSchedule(ctx context.Context, teamID, season int) ([]domain.MlbGame, error) {
	var raw struct {
		Dates []struct {
			Games []scheduleGame `json:"games"`
		} `json:"dates"`
	}
	q := url.Values{}
	q.Set("sportId", "1")
	q.Set("teamId", strconv.Itoa(teamID))
	q.Set("season", strconv.Itoa(season))
	if err := c.getJSON(ctx, "/v1/schedule", q, &raw); err != nil {
		return nil, err
	}
	out := []domain.MlbGame{}
	for _, d := range raw.Dates {
		for _, g := range d.Games {
			out = append(out, normalizeScheduleGame(g))
		}
	}
	return out, nil
}

type scheduleGame struct {
	GamePk       int    `json:"gamePk"`
	Season       string `json:"season"`
	GameDate     string `json:"gameDate"`
	OfficialDate string `json:"officialDate"`
	Status       struct {
		AbstractGameState string `json:"abstractGameState"`
		CodedGameState    string `json:"codedGameState"`
		DetailedState     string `json:"detailedState"`
	} `json:"status"`
	Teams struct {
		Away scheduleSide `json:"away"`
		Home scheduleSide `json:"home"`
	} `json:"teams"`
	Linescore *struct {
		CurrentInning int    `json:"currentInning"`
		IsTopInning   bool   `json:"isTopInning"`
		InningHalf    string `json:"inningHalf"`
	} `json:"linescore"`
}

type scheduleSide struct {
	Score *int `json:"score"`
	Team  struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Link string `json:"link"`
	} `json:"team"`
}

func normalizeScheduleGame(g scheduleGame) domain.MlbGame {
	season, _ := strconv.Atoi(g.Season)
	awayAbbr := ""
	homeAbbr := ""
	game := domain.MlbGame{
		ID:           g.GamePk,
		Season:       season,
		GameDate:     g.GameDate,
		OfficialDate: g.OfficialDate,
		Status:       mapStatus(g.Status.AbstractGameState, g.Status.CodedGameState, g.Status.DetailedState),
		StatusDetail: g.Status.DetailedState,
		AwayTeam: domain.MlbTeam{
			ID: g.Teams.Away.Team.ID, Name: g.Teams.Away.Team.Name,
			Abbreviation: awayAbbr, LogoURL: logoURL(g.Teams.Away.Team.ID),
		},
		HomeTeam: domain.MlbTeam{
			ID: g.Teams.Home.Team.ID, Name: g.Teams.Home.Team.Name,
			Abbreviation: homeAbbr, LogoURL: logoURL(g.Teams.Home.Team.ID),
		},
		AwayScore: g.Teams.Away.Score,
		HomeScore: g.Teams.Home.Score,
	}
	if g.Linescore != nil && g.Linescore.CurrentInning > 0 {
		inn := g.Linescore.CurrentInning
		game.CurrentInning = &inn
		if g.Linescore.IsTopInning || strings.EqualFold(g.Linescore.InningHalf, "Top") {
			game.CurrentInningHalf = "top"
		} else {
			game.CurrentInningHalf = "bottom"
		}
	}
	return game
}

func mapStatus(abstract, coded, detailed string) domain.MlbGameStatus {
	a := strings.ToLower(abstract)
	d := strings.ToLower(detailed)
	c := strings.ToUpper(coded)
	switch {
	case strings.Contains(d, "postpone"):
		return domain.MlbPostponed
	case strings.Contains(d, "cancel") || c == "C":
		return domain.MlbCancelled
	case a == "live" || c == "I":
		return domain.MlbLive
	case a == "final" || c == "F" || c == "O" || c == "D":
		return domain.MlbFinal
	case a == "preview":
		if strings.Contains(d, "pre-game") || strings.Contains(d, "warmup") {
			return domain.MlbPreGame
		}
		return domain.MlbScheduled
	default:
		return domain.MlbUnknown
	}
}

func (c *Client) GameDetail(ctx context.Context, gamePk int) (*domain.MlbGameDetail, error) {
	var raw liveFeed
	path := fmt.Sprintf("/v1.1/game/%d/feed/live", gamePk)
	if err := c.getJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return normalizeLiveFeed(raw), nil
}

type liveFeed struct {
	GamePk   int `json:"gamePk"`
	GameData struct {
		Game struct {
			Season string `json:"season"`
		} `json:"game"`
		Datetime struct {
			DateTime     string `json:"dateTime"`
			OfficialDate string `json:"officialDate"`
		} `json:"datetime"`
		Status struct {
			AbstractGameState string `json:"abstractGameState"`
			CodedGameState    string `json:"codedGameState"`
			DetailedState     string `json:"detailedState"`
		} `json:"status"`
		Teams struct {
			Away liveTeam `json:"away"`
			Home liveTeam `json:"home"`
		} `json:"teams"`
	} `json:"gameData"`
	LiveData struct {
		Linescore struct {
			CurrentInning int  `json:"currentInning"`
			IsTopInning   bool `json:"isTopInning"`
			Innings       []struct {
				Num  int `json:"num"`
				Home struct {
					Runs   *int `json:"runs"`
					Hits   int  `json:"hits"`
					Errors int  `json:"errors"`
				} `json:"home"`
				Away struct {
					Runs   *int `json:"runs"`
					Hits   int  `json:"hits"`
					Errors int  `json:"errors"`
				} `json:"away"`
			} `json:"innings"`
			Teams struct {
				Home struct {
					Runs   int `json:"runs"`
					Hits   int `json:"hits"`
					Errors int `json:"errors"`
				} `json:"home"`
				Away struct {
					Runs   int `json:"runs"`
					Hits   int `json:"hits"`
					Errors int `json:"errors"`
				} `json:"away"`
			} `json:"teams"`
		} `json:"linescore"`
		Plays struct {
			AllPlays []struct {
				Result struct {
					Event       string `json:"event"`
					Description string `json:"description"`
					AwayScore   int    `json:"awayScore"`
					HomeScore   int    `json:"homeScore"`
					RBI         int    `json:"rbi"`
				} `json:"result"`
				About struct {
					AtBatIndex    int    `json:"atBatIndex"`
					HalfInning    string `json:"halfInning"`
					Inning        int    `json:"inning"`
					IsScoringPlay bool   `json:"isScoringPlay"`
					EndTime       string `json:"endTime"`
				} `json:"about"`
			} `json:"allPlays"`
		} `json:"plays"`
		Boxscore struct {
			Teams struct {
				Away boxTeam `json:"away"`
				Home boxTeam `json:"home"`
			} `json:"teams"`
		} `json:"boxscore"`
	} `json:"liveData"`
}

type boxTeam struct {
	Team struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		Abbreviation string `json:"abbreviation"`
		TeamName     string `json:"teamName"`
	} `json:"team"`
	Batters  []int `json:"batters"`
	Pitchers []int `json:"pitchers"`
	Players  map[string]struct {
		Person struct {
			ID       int    `json:"id"`
			FullName string `json:"fullName"`
		} `json:"person"`
		Position struct {
			Abbreviation string `json:"abbreviation"`
		} `json:"position"`
		BattingOrder string `json:"battingOrder"`
		Stats        struct {
			Batting struct {
				Summary    string `json:"summary"`
				AtBats     int    `json:"atBats"`
				Runs       int    `json:"runs"`
				Hits       int    `json:"hits"`
				RBI        int    `json:"rbi"`
				BaseOnBalls int   `json:"baseOnBalls"`
				StrikeOuts int    `json:"strikeOuts"`
				HomeRuns   int    `json:"homeRuns"`
			} `json:"batting"`
			Pitching struct {
				Note           string `json:"note"`
				Summary        string `json:"summary"`
				InningsPitched string `json:"inningsPitched"`
				Hits           int    `json:"hits"`
				Runs           int    `json:"runs"`
				EarnedRuns     int    `json:"earnedRuns"`
				BaseOnBalls    int    `json:"baseOnBalls"`
				StrikeOuts     int    `json:"strikeOuts"`
				HomeRuns       int    `json:"homeRuns"`
				PitchesThrown  int    `json:"pitchesThrown"`
				Strikes        int    `json:"strikes"`
			} `json:"pitching"`
		} `json:"stats"`
	} `json:"players"`
}

type liveTeam struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	TeamName     string `json:"teamName"`
}

func normalizeLiveFeed(raw liveFeed) *domain.MlbGameDetail {
	season, _ := strconv.Atoi(raw.GameData.Game.Season)
	awayRuns := raw.LiveData.Linescore.Teams.Away.Runs
	homeRuns := raw.LiveData.Linescore.Teams.Home.Runs
	game := domain.MlbGame{
		ID:           raw.GamePk,
		Season:       season,
		GameDate:     raw.GameData.Datetime.DateTime,
		OfficialDate: raw.GameData.Datetime.OfficialDate,
		Status: mapStatus(
			raw.GameData.Status.AbstractGameState,
			raw.GameData.Status.CodedGameState,
			raw.GameData.Status.DetailedState,
		),
		StatusDetail: raw.GameData.Status.DetailedState,
		AwayTeam: domain.MlbTeam{
			ID: raw.GameData.Teams.Away.ID, Name: raw.GameData.Teams.Away.Name,
			Abbreviation: raw.GameData.Teams.Away.Abbreviation,
			ShortName:    raw.GameData.Teams.Away.TeamName,
			LogoURL:      logoURL(raw.GameData.Teams.Away.ID),
		},
		HomeTeam: domain.MlbTeam{
			ID: raw.GameData.Teams.Home.ID, Name: raw.GameData.Teams.Home.Name,
			Abbreviation: raw.GameData.Teams.Home.Abbreviation,
			ShortName:    raw.GameData.Teams.Home.TeamName,
			LogoURL:      logoURL(raw.GameData.Teams.Home.ID),
		},
		AwayScore: &awayRuns,
		HomeScore: &homeRuns,
	}
	if raw.LiveData.Linescore.CurrentInning > 0 {
		inn := raw.LiveData.Linescore.CurrentInning
		game.CurrentInning = &inn
		if raw.LiveData.Linescore.IsTopInning {
			game.CurrentInningHalf = "top"
		} else {
			game.CurrentInningHalf = "bottom"
		}
	}

	innings := make([]domain.MlbInning, 0, len(raw.LiveData.Linescore.Innings))
	for _, in := range raw.LiveData.Linescore.Innings {
		ar, hr := 0, 0
		if in.Away.Runs != nil {
			ar = *in.Away.Runs
		}
		if in.Home.Runs != nil {
			hr = *in.Home.Runs
		}
		innings = append(innings, domain.MlbInning{
			Number: in.Num, AwayRuns: ar, HomeRuns: hr,
			AwayHits: in.Away.Hits, HomeHits: in.Home.Hits,
			AwayErrors: in.Away.Errors, HomeErrors: in.Home.Errors,
		})
	}

	plays := make([]domain.MlbPlay, 0, len(raw.LiveData.Plays.AllPlays))
	for _, p := range raw.LiveData.Plays.AllPlays {
		half := "bottom"
		if strings.EqualFold(p.About.HalfInning, "top") {
			half = "top"
		}
		away := p.Result.AwayScore
		home := p.Result.HomeScore
		idx := p.About.AtBatIndex
		plays = append(plays, domain.MlbPlay{
			ID:            fmt.Sprintf("%d-%d-%s", p.About.Inning, p.About.AtBatIndex, half),
			Inning:        p.About.Inning,
			Half:          half,
			Event:         p.Result.Event,
			Description:   p.Result.Description,
			IsScoringPlay: p.About.IsScoringPlay || p.Result.RBI > 0,
			AwayScore:     &away,
			HomeScore:     &home,
			AtBatIndex:    &idx,
		})
	}

	awayBox := normalizeBoxTeam(raw.LiveData.Boxscore.Teams.Away, game.AwayTeam)
	homeBox := normalizeBoxTeam(raw.LiveData.Boxscore.Teams.Home, game.HomeTeam)

	return &domain.MlbGameDetail{
		Game:       game,
		Innings:    innings,
		Plays:      plays,
		AwayHits:   raw.LiveData.Linescore.Teams.Away.Hits,
		HomeHits:   raw.LiveData.Linescore.Teams.Home.Hits,
		AwayErrors: raw.LiveData.Linescore.Teams.Away.Errors,
		HomeErrors: raw.LiveData.Linescore.Teams.Home.Errors,
		AwayBox:    awayBox,
		HomeBox:    homeBox,
	}
}

func normalizeBoxTeam(raw boxTeam, fallback domain.MlbTeam) *domain.MlbTeamBox {
	team := fallback
	if raw.Team.ID > 0 {
		team = domain.MlbTeam{
			ID:           raw.Team.ID,
			Name:         firstNonEmpty(raw.Team.Name, fallback.Name),
			Abbreviation: firstNonEmpty(raw.Team.Abbreviation, fallback.Abbreviation),
			ShortName:    firstNonEmpty(raw.Team.TeamName, fallback.ShortName),
			LogoURL:      logoURL(raw.Team.ID),
		}
	}
	playerKey := func(id int) string { return fmt.Sprintf("ID%d", id) }

	batters := make([]domain.MlbBatterLine, 0, len(raw.Batters))
	for _, id := range raw.Batters {
		p, ok := raw.Players[playerKey(id)]
		if !ok {
			continue
		}
		b := p.Stats.Batting
		// Skip pitchers with empty batting lines that never appeared (0 PA-ish and no summary)
		if b.AtBats == 0 && b.Hits == 0 && b.Runs == 0 && b.RBI == 0 && b.BaseOnBalls == 0 &&
			b.StrikeOuts == 0 && b.Summary == "" && p.BattingOrder == "" {
			continue
		}
		order := 0
		if p.BattingOrder != "" {
			if n, err := strconv.Atoi(p.BattingOrder); err == nil {
				order = n / 100 // MLB uses 100,200,... for slots
				if order == 0 {
					order = n
				}
			}
		}
		batters = append(batters, domain.MlbBatterLine{
			PlayerID:     p.Person.ID,
			Name:         p.Person.FullName,
			Position:     p.Position.Abbreviation,
			BattingOrder: order,
			AtBats:       b.AtBats,
			Runs:         b.Runs,
			Hits:         b.Hits,
			RBI:          b.RBI,
			Walks:        b.BaseOnBalls,
			StrikeOuts:   b.StrikeOuts,
			HomeRuns:     b.HomeRuns,
			Summary:      b.Summary,
		})
	}

	pitchers := make([]domain.MlbPitcherLine, 0, len(raw.Pitchers))
	for _, id := range raw.Pitchers {
		p, ok := raw.Players[playerKey(id)]
		if !ok {
			continue
		}
		pit := p.Stats.Pitching
		if pit.InningsPitched == "" && pit.PitchesThrown == 0 && pit.Summary == "" {
			continue
		}
		pitchers = append(pitchers, domain.MlbPitcherLine{
			PlayerID:       p.Person.ID,
			Name:           p.Person.FullName,
			Note:           pit.Note,
			InningsPitched: pit.InningsPitched,
			Hits:           pit.Hits,
			Runs:           pit.Runs,
			EarnedRuns:     pit.EarnedRuns,
			Walks:          pit.BaseOnBalls,
			StrikeOuts:     pit.StrikeOuts,
			HomeRuns:       pit.HomeRuns,
			PitchesThrown:  pit.PitchesThrown,
			Strikes:        pit.Strikes,
			Summary:        pit.Summary,
		})
	}

	if len(batters) == 0 && len(pitchers) == 0 {
		return nil
	}
	return &domain.MlbTeamBox{Team: team, Batters: batters, Pitchers: pitchers}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (c *Client) Standings(ctx context.Context, season int) (*domain.MlbStandings, error) {
	if season <= 0 {
		season = time.Now().Year()
	}
	divNames := map[int]string{}
	var divRaw struct {
		Divisions []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"divisions"`
	}
	dq := url.Values{}
	dq.Set("sportId", "1")
	if err := c.getJSON(ctx, "/v1/divisions", dq, &divRaw); err == nil {
		for _, d := range divRaw.Divisions {
			divNames[d.ID] = d.Name
		}
	}

	var raw struct {
		Records []struct {
			StandingsType string `json:"standingsType"`
			League        struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"league"`
			Division struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"division"`
			TeamRecords []struct {
				Team struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"team"`
				Wins              int    `json:"wins"`
				Losses            int    `json:"losses"`
				WinningPercentage string `json:"winningPercentage"`
				GamesBack         string `json:"gamesBack"`
				WildCardGamesBack string `json:"wildCardGamesBack"`
				RunDifferential   int    `json:"runDifferential"`
				DivisionRank      string `json:"divisionRank"`
				WildCardRank      string `json:"wildCardRank"`
				DivisionLeader    bool   `json:"divisionLeader"`
				Clinched          bool   `json:"clinched"`
				Streak            struct {
					StreakCode string `json:"streakCode"`
				} `json:"streak"`
			} `json:"teamRecords"`
		} `json:"records"`
	}
	q := url.Values{}
	q.Set("leagueId", "103,104")
	q.Set("season", strconv.Itoa(season))
	q.Set("standingsTypes", "regularSeason,wildCard")
	if err := c.getJSON(ctx, "/v1/standings", q, &raw); err != nil {
		return nil, err
	}

	leagueLabel := func(id int) string {
		switch id {
		case 103:
			return "AL"
		case 104:
			return "NL"
		default:
			return fmt.Sprintf("L%d", id)
		}
	}
	shortDiv := func(full string) string {
		full = strings.TrimPrefix(full, "American League ")
		full = strings.TrimPrefix(full, "National League ")
		return full
	}

	sections := make([]domain.MlbStandingSection, 0, len(raw.Records))
	for _, rec := range raw.Records {
		kind := "division"
		name := rec.Division.Name
		if name == "" {
			name = divNames[rec.Division.ID]
		}
		id := fmt.Sprintf("div-%d", rec.Division.ID)
		if rec.StandingsType == "wildCard" {
			kind = "wildcard"
			name = "Wild Card"
			id = fmt.Sprintf("wc-%d", rec.League.ID)
		} else {
			name = shortDiv(name)
			if name == "" {
				name = fmt.Sprintf("Division %d", rec.Division.ID)
			}
		}
		rows := make([]domain.MlbStandingRow, 0, len(rec.TeamRecords))
		for i, tr := range rec.TeamRecords {
			rank := i + 1
			if kind == "wildcard" && tr.WildCardRank != "" {
				if n, err := strconv.Atoi(tr.WildCardRank); err == nil {
					rank = n
				}
			} else if tr.DivisionRank != "" {
				if n, err := strconv.Atoi(tr.DivisionRank); err == nil {
					rank = n
				}
			}
			abbr := ""
			// Prefer abbreviation from teams cache if present
			rows = append(rows, domain.MlbStandingRow{
				Rank:              rank,
				Team: domain.MlbTeam{
					ID:           tr.Team.ID,
					Name:         tr.Team.Name,
					Abbreviation: abbr,
					LogoURL:      logoURL(tr.Team.ID),
				},
				Wins:              tr.Wins,
				Losses:            tr.Losses,
				WinningPercentage: tr.WinningPercentage,
				GamesBack:         tr.GamesBack,
				WildCardGamesBack: tr.WildCardGamesBack,
				RunDifferential:   tr.RunDifferential,
				Streak:            tr.Streak.StreakCode,
				DivisionLeader:    tr.DivisionLeader,
				Clinched:          tr.Clinched,
			})
		}
		sections = append(sections, domain.MlbStandingSection{
			ID:     id,
			League: leagueLabel(rec.League.ID),
			Name:   name,
			Kind:   kind,
			Teams:  rows,
		})
	}

	// Stable order: AL East/Central/West, AL WC, NL East/Central/West, NL WC
	order := []string{"div-201", "div-202", "div-200", "wc-103", "div-204", "div-205", "div-203", "wc-104"}
	byID := map[string]domain.MlbStandingSection{}
	for _, s := range sections {
		byID[s.ID] = s
	}
	ordered := make([]domain.MlbStandingSection, 0, len(order))
	for _, id := range order {
		if s, ok := byID[id]; ok {
			ordered = append(ordered, s)
			delete(byID, id)
		}
	}
	for _, s := range sections {
		if _, ok := byID[s.ID]; ok {
			ordered = append(ordered, s)
			delete(byID, s.ID)
		}
	}

	// Fill abbreviations from team list when available
	if teams, err := c.ListTeams(ctx); err == nil {
		abbr := map[int]string{}
		short := map[int]string{}
		for _, t := range teams {
			abbr[t.ID] = t.Abbreviation
			short[t.ID] = t.ShortName
		}
		for i := range ordered {
			for j := range ordered[i].Teams {
				id := ordered[i].Teams[j].Team.ID
				ordered[i].Teams[j].Team.Abbreviation = abbr[id]
				ordered[i].Teams[j].Team.ShortName = short[id]
			}
		}
	}

	return &domain.MlbStandings{Season: season, Sections: ordered}, nil
}
