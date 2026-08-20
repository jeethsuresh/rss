package pandascore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const baseURL = "https://api.pandascore.co"

// Free tier is ~1000 req/hour; pace conservatively (~1 req / 1.2s ≈ 3000/hr max theoretical; stay under).
const minGap = 1200 * time.Millisecond

type Client struct {
	HTTP  *http.Client
	Token string

	rateMu  sync.Mutex
	lastReq time.Time
}

func NewClientFromEnv() *Client {
	tok := strings.TrimSpace(os.Getenv("PANDASCORE_API_TOKEN"))
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("PANDASCORE_TOKEN"))
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

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, dest any) error {
	if !c.Configured() {
		return fmt.Errorf("%w: PandaScore token not configured (PANDASCORE_API_TOKEN)", domain.ErrInvalidParams)
	}
	u := baseURL + path
	if q == nil {
		q = url.Values{}
	}
	u += "?" + q.Encode()
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.throttle(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; PandaScore)")
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
			lastErr = fmt.Errorf("%w: pandascore rate limited", domain.ErrNetwork)
			retryAfter := (attempt + 1) * 2
			if ra := res.Header.Get("Retry-After"); ra != "" {
				if n, err := strconv.Atoi(ra); err == nil && n > 0 {
					retryAfter = n
				}
			}
			timer := time.NewTimer(time.Duration(retryAfter) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: pandascore auth failed (%d)", domain.ErrInvalidParams, res.StatusCode)
		}
		if res.StatusCode == http.StatusNotFound {
			return domain.ErrNotFound
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("%w: pandascore HTTP %d: %s", domain.ErrNetwork, res.StatusCode, truncate(string(body), 200))
		}
		if err := json.Unmarshal(body, dest); err != nil {
			return fmt.Errorf("%w: pandascore decode: %v", domain.ErrParse, err)
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%w: pandascore failed", domain.ErrNetwork)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func MapTier(raw string) domain.DotaEventTier {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "s", "s-tier", "premier":
		return domain.DotaTierPremier
	case "a", "a-tier", "professional":
		return domain.DotaTierProfessional
	case "b", "b-tier", "semi-pro", "semipro":
		return domain.DotaTierSemiPro
	case "c", "c-tier", "d", "d-tier", "amateur":
		return domain.DotaTierAmateur
	default:
		return domain.DotaTierUnknown
	}
}

func MapEventStatus(beginAt, endAt *string, rawStatus string) domain.DotaEventStatus {
	st := strings.ToLower(rawStatus)
	switch st {
	case "running", "ongoing":
		return domain.DotaEventOngoing
	case "finished", "completed", "canceled", "cancelled":
		return domain.DotaEventCompleted
	case "not_started", "upcoming":
		return domain.DotaEventUpcoming
	}
	now := time.Now().UTC()
	if beginAt != nil {
		if t, err := time.Parse(time.RFC3339, *beginAt); err == nil {
			if endAt != nil {
				if e, err := time.Parse(time.RFC3339, *endAt); err == nil {
					if now.After(e) {
						return domain.DotaEventCompleted
					}
					if now.After(t) && now.Before(e) {
						return domain.DotaEventOngoing
					}
				}
			}
			if now.Before(t) {
				return domain.DotaEventUpcoming
			}
		}
	}
	return domain.DotaEventUpcoming
}

func MapMatchStatus(raw string) domain.DotaMatchStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running", "live":
		return domain.DotaMatchLive
	case "finished", "completed", "canceled", "cancelled":
		return domain.DotaMatchCompleted
	default:
		return domain.DotaMatchUpcoming
	}
}

type psLeague struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
	Slug     string `json:"slug"`
}

type psSerie struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	FullName   string   `json:"full_name"`
	Year       int      `json:"year"`
	BeginAt    *string  `json:"begin_at"`
	EndAt      *string  `json:"end_at"`
	Tier       string   `json:"tier"`
	LeagueID   int      `json:"league_id"`
	League     *psLeague `json:"league"`
	WinnerID   *int     `json:"winner_id"`
}

type psTeam struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Acronym  string `json:"acronym"`
	ImageURL string `json:"image_url"`
}

type psOpponent struct {
	Type     string  `json:"type"`
	Opponent *psTeam `json:"opponent"`
}

type psGame struct {
	ID       int     `json:"id"`
	Position int     `json:"position"`
	Status   string  `json:"status"`
	Length   *int    `json:"length"`
	BeginAt  *string `json:"begin_at"`
	Complete bool    `json:"complete"`
	Finished bool    `json:"finished"`
	Winner   *struct {
		ID   *int   `json:"id"`
		Type string `json:"type"`
	} `json:"winner"`
	// Some payloads expose steam / detailed ids under varying keys — captured via raw map later if needed.
}

type psMatch struct {
	ID               int          `json:"id"`
	Name             string       `json:"name"`
	Status           string       `json:"status"`
	BeginAt          *string      `json:"begin_at"`
	ScheduledAt      *string      `json:"scheduled_at"`
	NumberOfGames    int          `json:"number_of_games"`
	Opponents        []psOpponent `json:"opponents"`
	Results          []struct {
		TeamID int `json:"team_id"`
		Score  int `json:"score"`
	} `json:"results"`
	SerieID      int     `json:"serie_id"`
	TournamentID int     `json:"tournament_id"`
	Serie        *psSerie `json:"serie"`
	Tournament   *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"tournament"`
	Games []psGame `json:"games"`
	Year  int      `json:"year"`
}

func teamFromPS(t *psTeam) domain.DotaTeam {
	if t == nil {
		return domain.DotaTeam{Name: "TBD"}
	}
	return domain.DotaTeam{
		ID:        t.ID,
		Name:      t.Name,
		ShortName: t.Acronym,
		LogoURL:   t.ImageURL,
	}
}

func eventFromSerie(s psSerie) domain.DotaEvent {
	name := strings.TrimSpace(s.FullName)
	if name == "" {
		name = strings.TrimSpace(s.Name)
	}
	leagueName := ""
	logo := ""
	leagueID := ""
	if s.League != nil {
		leagueName = s.League.Name
		logo = s.League.ImageURL
		leagueID = strconv.Itoa(s.League.ID)
	} else if s.LeagueID > 0 {
		leagueID = strconv.Itoa(s.LeagueID)
	}
	year := s.Year
	if year == 0 && s.BeginAt != nil {
		if t, err := time.Parse(time.RFC3339, *s.BeginAt); err == nil {
			year = t.Year()
		}
	}
	return domain.DotaEvent{
		ID:         strconv.Itoa(s.ID),
		Name:       name,
		Type:       domain.DotaEventTournament,
		Tier:       MapTier(s.Tier),
		Status:     MapEventStatus(s.BeginAt, s.EndAt, ""),
		StartAt:    s.BeginAt,
		EndAt:      s.EndAt,
		LogoURL:    logo,
		LeagueID:   leagueID,
		LeagueName: leagueName,
		Year:       year,
		Organizer:  leagueName,
	}
}

func matchFromPS(m psMatch, yearFallback int) domain.DotaMatch {
	var a, b domain.DotaTeam
	if len(m.Opponents) > 0 && m.Opponents[0].Opponent != nil {
		a = teamFromPS(m.Opponents[0].Opponent)
	} else {
		a = domain.DotaTeam{Name: "TBD"}
	}
	if len(m.Opponents) > 1 && m.Opponents[1].Opponent != nil {
		b = teamFromPS(m.Opponents[1].Opponent)
	} else {
		b = domain.DotaTeam{Name: "TBD"}
	}
	var scoreA, scoreB *int
	for _, r := range m.Results {
		sc := r.Score
		if r.TeamID == a.ID {
			scoreA = &sc
		} else if r.TeamID == b.ID {
			scoreB = &sc
		}
	}
	scheduled := m.ScheduledAt
	if scheduled == nil {
		scheduled = m.BeginAt
	}
	eventID := ""
	eventName := ""
	if m.SerieID > 0 {
		eventID = strconv.Itoa(m.SerieID)
	}
	if m.Serie != nil {
		eventID = strconv.Itoa(m.Serie.ID)
		eventName = m.Serie.FullName
		if eventName == "" {
			eventName = m.Serie.Name
		}
	}
	stage := ""
	if m.Tournament != nil {
		stage = m.Tournament.Name
	}
	year := m.Year
	if year == 0 && m.Serie != nil && m.Serie.Year > 0 {
		year = m.Serie.Year
	}
	if year == 0 {
		year = yearFallback
	}
	bo := m.NumberOfGames
	var bestOf *int
	if bo > 0 {
		bestOf = &bo
	}
	return domain.DotaMatch{
		ID:          m.ID,
		EventID:     eventID,
		EventName:   eventName,
		TeamA:       a,
		TeamB:       b,
		ScheduledAt: scheduled,
		Status:      MapMatchStatus(m.Status),
		BestOf:      bestOf,
		ScoreA:      scoreA,
		ScoreB:      scoreB,
		Year:        year,
		Stage:       stage,
	}
}

func gamesFromPS(matchID int, games []psGame, teamA, teamB domain.DotaTeam) []domain.DotaGame {
	out := make([]domain.DotaGame, 0, len(games))
	for _, g := range games {
		idx := g.Position
		if idx <= 0 {
			idx = len(out) + 1
		}
		dg := domain.DotaGame{
			ID:                strconv.Itoa(g.ID),
			MatchID:           matchID,
			GameIndex:         idx,
			DurationSeconds:   g.Length,
			MappingConfidence: "unknown",
			DetailAvailable:   false,
		}
		if g.Winner != nil && g.Winner.ID != nil {
			wid := *g.Winner.ID
			if wid == teamA.ID {
				// Winner side unknown without radiant/dire — leave Winner empty; set teams for context.
				dg.RadiantTeam = &teamA
				dg.DireTeam = &teamB
			} else if wid == teamB.ID {
				dg.RadiantTeam = &teamA
				dg.DireTeam = &teamB
			}
		}
		out = append(out, dg)
	}
	return out
}

func (c *Client) ListSeriesForYear(ctx context.Context, year int) ([]domain.DotaEvent, error) {
	if year <= 0 {
		year = time.Now().Year()
	}
	start := fmt.Sprintf("%d-01-01T00:00:00Z", year)
	end := fmt.Sprintf("%d-12-31T23:59:59Z", year)
	q := url.Values{}
	q.Set("range[begin_at]", start+","+end)
	q.Set("sort", "-begin_at")
	q.Set("per_page", "100")
	q.Set("page", "1")
	var raw []psSerie
	if err := c.getJSON(ctx, "/dota2/series", q, &raw); err != nil {
		// PandaScore sometimes 404s empty year windows; treat as no events.
		if errors.Is(err, domain.ErrNotFound) {
			return []domain.DotaEvent{}, nil
		}
		return nil, err
	}
	out := make([]domain.DotaEvent, 0, len(raw))
	for _, s := range raw {
		ev := eventFromSerie(s)
		if ev.Year == 0 {
			ev.Year = year
		}
		out = append(out, ev)
	}
	return out, nil
}

func (c *Client) GetSerie(ctx context.Context, serieID int) (*domain.DotaEvent, error) {
	var raw psSerie
	if err := c.getJSON(ctx, "/dota2/series/"+strconv.Itoa(serieID), nil, &raw); err != nil {
		return nil, err
	}
	ev := eventFromSerie(raw)
	return &ev, nil
}

func (c *Client) ListSerieMatches(ctx context.Context, serieID int) ([]domain.DotaMatch, error) {
	q := url.Values{}
	q.Set("per_page", "100")
	q.Set("sort", "-begin_at")
	var raw []psMatch
	if err := c.getJSON(ctx, "/dota2/series/"+strconv.Itoa(serieID)+"/matches", q, &raw); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return []domain.DotaMatch{}, nil
		}
		return nil, err
	}
	out := make([]domain.DotaMatch, 0, len(raw))
	for _, m := range raw {
		out = append(out, matchFromPS(m, 0))
	}
	return out, nil
}

func (c *Client) GetMatch(ctx context.Context, matchID int) (*domain.DotaMatchDetail, error) {
	var raw psMatch
	if err := c.getJSON(ctx, "/dota2/matches/"+strconv.Itoa(matchID), nil, &raw); err != nil {
		return nil, err
	}
	m := matchFromPS(raw, 0)
	games := gamesFromPS(m.ID, raw.Games, m.TeamA, m.TeamB)
	return &domain.DotaMatchDetail{
		Match: m,
		Games: games,
		Live:  m.Status == domain.DotaMatchLive,
	}, nil
}

func (c *Client) ListTeamMatches(ctx context.Context, teamID int, year int) ([]domain.DotaMatch, error) {
	q := url.Values{}
	q.Set("filter[opponent_id]", strconv.Itoa(teamID))
	q.Set("per_page", "50")
	q.Set("sort", "-begin_at")
	if year > 0 {
		start := fmt.Sprintf("%d-01-01T00:00:00Z", year)
		end := fmt.Sprintf("%d-12-31T23:59:59Z", year)
		q.Set("range[begin_at]", start+","+end)
	}
	var raw []psMatch
	if err := c.getJSON(ctx, "/dota2/matches", q, &raw); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return []domain.DotaMatch{}, nil
		}
		return nil, err
	}
	out := make([]domain.DotaMatch, 0, len(raw))
	for _, m := range raw {
		out = append(out, matchFromPS(m, year))
	}
	return out, nil
}

func (c *Client) SearchTeams(ctx context.Context, query string) ([]domain.DotaTeam, error) {
	q := url.Values{}
	q.Set("search[name]", strings.TrimSpace(query))
	q.Set("per_page", "25")
	var raw []psTeam
	if err := c.getJSON(ctx, "/dota2/teams", q, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.DotaTeam, 0, len(raw))
	for i := range raw {
		out = append(out, teamFromPS(&raw[i]))
	}
	return out, nil
}

func (c *Client) GetTeam(ctx context.Context, teamID int) (*domain.DotaTeam, error) {
	var raw psTeam
	if err := c.getJSON(ctx, "/dota2/teams/"+strconv.Itoa(teamID), nil, &raw); err != nil {
		return nil, err
	}
	t := teamFromPS(&raw)
	return &t, nil
}
