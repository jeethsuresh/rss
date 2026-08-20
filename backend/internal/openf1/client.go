package openf1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const baseURL = "https://api.openf1.org/v1"

type Client struct {
	HTTP *http.Client

	mu         sync.Mutex
	yearsCache []int
	yearsAt    time.Time

	rateMu  sync.Mutex
	lastReq time.Time
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 45 * time.Second}}
}

// OpenF1 allows ~3 requests/second; serialize and pace calls.
func (c *Client) throttle(ctx context.Context) error {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	wait := 350*time.Millisecond - time.Since(c.lastReq)
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
	u := baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
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
		req.Header.Set("User-Agent", "RSSReader/0.1 (+local desktop; OpenF1)")
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
			lastErr = fmt.Errorf("%w: openf1 rate limited", domain.ErrNetwork)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 400 * time.Millisecond):
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("%w: openf1 status %d", domain.ErrNetwork, res.StatusCode)
		}
		return json.Unmarshal(body, dest)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%w: openf1 retries exhausted", domain.ErrNetwork)
}

func (c *Client) ListYears(ctx context.Context) ([]domain.F1Season, error) {
	c.mu.Lock()
	if time.Since(c.yearsAt) < 12*time.Hour && len(c.yearsCache) > 0 {
		years := append([]int{}, c.yearsCache...)
		c.mu.Unlock()
		out := make([]domain.F1Season, 0, len(years))
		for _, y := range years {
			out = append(out, domain.F1Season{Year: y})
		}
		return out, nil
	}
	c.mu.Unlock()

	var meetings []struct {
		Year int `json:"year"`
	}
	// OpenF1 has data roughly from 2023+. Probe a small year window.
	yearsSet := map[int]bool{}
	nowY := time.Now().UTC().Year()
	for y := nowY; y >= 2023; y-- {
		q := url.Values{}
		q.Set("year", strconv.Itoa(y))
		meetings = meetings[:0]
		if err := c.getJSON(ctx, "/meetings", q, &meetings); err != nil {
			continue
		}
		if len(meetings) > 0 {
			yearsSet[y] = true
		}
	}
	{
		y := nowY + 1
		q := url.Values{}
		q.Set("year", strconv.Itoa(y))
		meetings = meetings[:0]
		if err := c.getJSON(ctx, "/meetings", q, &meetings); err == nil && len(meetings) > 0 {
			yearsSet[y] = true
		}
	}
	years := make([]int, 0, len(yearsSet))
	for y := range yearsSet {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	c.mu.Lock()
	c.yearsCache = years
	c.yearsAt = time.Now()
	c.mu.Unlock()
	out := make([]domain.F1Season, 0, len(years))
	for _, y := range years {
		out = append(out, domain.F1Season{Year: y})
	}
	return out, nil
}

type meeting struct {
	MeetingKey           int    `json:"meeting_key"`
	MeetingName          string `json:"meeting_name"`
	MeetingOfficialName  string `json:"meeting_official_name"`
	Location             string `json:"location"`
	CountryName          string `json:"country_name"`
	CountryCode          string `json:"country_code"`
	CircuitShortName     string `json:"circuit_short_name"`
	DateStart            string `json:"date_start"`
	DateEnd              string `json:"date_end"`
	Year                 int    `json:"year"`
	IsCancelled          bool   `json:"is_cancelled"`
}

type session struct {
	SessionKey       int    `json:"session_key"`
	SessionType      string `json:"session_type"`
	SessionName      string `json:"session_name"`
	DateStart        string `json:"date_start"`
	DateEnd          string `json:"date_end"`
	MeetingKey       int    `json:"meeting_key"`
	CircuitShortName string `json:"circuit_short_name"`
	CountryName      string `json:"country_name"`
	CountryCode      string `json:"country_code"`
	Location         string `json:"location"`
	Year             int    `json:"year"`
	IsCancelled      bool   `json:"is_cancelled"`
}

func (c *Client) ListRaces(ctx context.Context, year int) ([]domain.F1Race, error) {
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	q := url.Values{}
	q.Set("year", strconv.Itoa(year))
	var meetings []meeting
	if err := c.getJSON(ctx, "/meetings", q, &meetings); err != nil {
		return nil, err
	}
	byKey := map[int]meeting{}
	for _, m := range meetings {
		byKey[m.MeetingKey] = m
	}

	sq := url.Values{}
	sq.Set("year", strconv.Itoa(year))
	var sessions []session
	if err := c.getJSON(ctx, "/sessions", sq, &sessions); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	byMeeting := map[int][]session{}
	for _, s := range sessions {
		byMeeting[s.MeetingKey] = append(byMeeting[s.MeetingKey], s)
	}

	out := make([]domain.F1Race, 0, len(byMeeting))
	for meetingKey, sess := range byMeeting {
		m := byKey[meetingKey]
		name := m.MeetingName
		if name == "" && len(sess) > 0 {
			name = sess[0].Location + " Grand Prix"
		}
		if strings.Contains(strings.ToLower(name), "pre-season") ||
			strings.Contains(strings.ToLower(name), "testing") {
			continue
		}
		sort.Slice(sess, func(i, j int) bool {
			return sess[i].DateStart < sess[j].DateStart
		})
		sessionRefs := make([]domain.F1Session, 0, len(sess))
		var primary *session
		for i := range sess {
			s := sess[i]
			ref := toF1Session(s, m.IsCancelled, now)
			if ref.Kind == domain.F1KindOther {
				continue
			}
			sessionRefs = append(sessionRefs, ref)
			if s.SessionName == "Race" {
				cp := s
				primary = &cp
			}
		}
		if len(sessionRefs) == 0 {
			continue
		}
		if primary == nil {
			// Prefer race-kind session, else last session of weekend.
			for i := range sess {
				if sessionKind(sess[i].SessionName) == domain.F1KindRace {
					cp := sess[i]
					primary = &cp
					break
				}
			}
		}
		if primary == nil {
			cp := sess[len(sess)-1]
			primary = &cp
		}
		race := domain.F1Race{
			MeetingKey:       meetingKey,
			SessionKey:       primary.SessionKey,
			Year:             firstNonZero(primary.Year, m.Year, year),
			Name:             name,
			OfficialName:     m.MeetingOfficialName,
			Location:         firstNonEmpty(primary.Location, m.Location),
			CountryName:      firstNonEmpty(primary.CountryName, m.CountryName),
			CountryCode:      firstNonEmpty(primary.CountryCode, m.CountryCode),
			CircuitShortName: firstNonEmpty(primary.CircuitShortName, m.CircuitShortName),
			DateStart:        primary.DateStart,
			DateEnd:          primary.DateEnd,
			Status:           raceStatus(primary.DateStart, primary.DateEnd, primary.IsCancelled || m.IsCancelled, now),
			Sessions:         sessionRefs,
		}
		out = append(out, race)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DateStart > out[j].DateStart
	})
	return out, nil
}

func sessionKind(sessionName string) domain.F1SessionKind {
	switch strings.TrimSpace(sessionName) {
	case "Practice 1", "Practice 2", "Practice 3", "Practice":
		return domain.F1KindPractice
	case "Sprint Qualifying", "Sprint Shootout":
		return domain.F1KindSprintQuali
	case "Qualifying":
		return domain.F1KindQuali
	case "Sprint":
		return domain.F1KindSprint
	case "Race":
		return domain.F1KindRace
	default:
		return domain.F1KindOther
	}
}

func toF1Session(s session, meetingCancelled bool, now time.Time) domain.F1Session {
	return domain.F1Session{
		SessionKey:  s.SessionKey,
		SessionName: s.SessionName,
		SessionType: s.SessionType,
		Kind:        sessionKind(s.SessionName),
		DateStart:   s.DateStart,
		DateEnd:     s.DateEnd,
		Status:      raceStatus(s.DateStart, s.DateEnd, s.IsCancelled || meetingCancelled, now),
	}
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func raceStatus(start, end string, cancelled bool, now time.Time) domain.F1RaceStatus {
	if cancelled {
		return domain.F1Cancelled
	}
	st, err1 := time.Parse(time.RFC3339, start)
	en, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil {
		st, _ = time.Parse("2006-01-02T15:04:05-07:00", start)
	}
	if err2 != nil {
		en, _ = time.Parse("2006-01-02T15:04:05-07:00", end)
	}
	if !st.IsZero() && now.Before(st) {
		return domain.F1Scheduled
	}
	if !en.IsZero() && now.After(en) {
		return domain.F1Completed
	}
	if !st.IsZero() && (en.IsZero() || !now.After(en)) && !now.Before(st) {
		return domain.F1InProgress
	}
	if !en.IsZero() && now.After(en) {
		return domain.F1Completed
	}
	return domain.F1Scheduled
}

func (c *Client) RaceDetail(ctx context.Context, sessionKey int) (*domain.F1RaceDetail, error) {
	if sessionKey <= 0 {
		return nil, domain.ErrInvalidParams
	}
	q := url.Values{}
	q.Set("session_key", strconv.Itoa(sessionKey))
	var sessions []session
	if err := c.getJSON(ctx, "/sessions", q, &sessions); err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, domain.ErrNotFound
	}
	s := sessions[0]

	mq := url.Values{}
	mq.Set("meeting_key", strconv.Itoa(s.MeetingKey))
	var meetings []meeting
	_ = c.getJSON(ctx, "/meetings", mq, &meetings)
	var m meeting
	if len(meetings) > 0 {
		m = meetings[0]
	}

	driversByNum := map[int]struct {
		Name, Acronym, Team string
	}{}
	var drivers []struct {
		DriverNumber int    `json:"driver_number"`
		FullName     string `json:"full_name"`
		NameAcronym  string `json:"name_acronym"`
		TeamName     string `json:"team_name"`
	}
	_ = c.getJSON(ctx, "/drivers", q, &drivers)
	for _, d := range drivers {
		driversByNum[d.DriverNumber] = struct{ Name, Acronym, Team string }{
			Name: d.FullName, Acronym: d.NameAcronym, Team: d.TeamName,
		}
	}

	var resultsRaw []struct {
		Position     int     `json:"position"`
		DriverNumber int     `json:"driver_number"`
		NumberOfLaps int     `json:"number_of_laps"`
		Points       float64 `json:"points"`
		DNF          bool    `json:"dnf"`
		DNS          bool    `json:"dns"`
		DSQ          bool    `json:"dsq"`
		Duration     *float64 `json:"duration"`
		GapToLeader  any     `json:"gap_to_leader"`
	}
	_ = c.getJSON(ctx, "/session_result", q, &resultsRaw)

	results := make([]domain.F1DriverResult, 0, len(resultsRaw))
	for _, r := range resultsRaw {
		info := driversByNum[r.DriverNumber]
		name := info.Name
		if name == "" {
			name = fmt.Sprintf("Driver #%d", r.DriverNumber)
		}
		results = append(results, domain.F1DriverResult{
			Position:     r.Position,
			DriverNumber: r.DriverNumber,
			Name:         name,
			NameAcronym:  info.Acronym,
			TeamName:     info.Team,
			Points:       r.Points,
			Laps:         r.NumberOfLaps,
			DNF:          r.DNF,
			DNS:          r.DNS,
			DSQ:          r.DSQ,
			GapToLeader:  formatGap(r.GapToLeader),
			DurationSec:  r.Duration,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Position < results[j].Position
	})

	var control []struct {
		Date         string `json:"date"`
		Category     string `json:"category"`
		Flag         string `json:"flag"`
		Scope        string `json:"scope"`
		LapNumber    *int   `json:"lap_number"`
		DriverNumber *int   `json:"driver_number"`
		Message      string `json:"message"`
	}
	_ = c.getJSON(ctx, "/race_control", q, &control)

	events := make([]domain.F1Event, 0, len(control))
	for i, e := range control {
		sig := isSignificantEvent(e.Category, e.Flag, e.Message)
		driverName := ""
		if e.DriverNumber != nil {
			if info, ok := driversByNum[*e.DriverNumber]; ok {
				driverName = info.Name
				if driverName == "" {
					driverName = info.Acronym
				}
			}
		}
		events = append(events, domain.F1Event{
			ID:           fmt.Sprintf("%d-%d", sessionKey, i),
			Date:         e.Date,
			Category:     e.Category,
			Flag:         e.Flag,
			Scope:        e.Scope,
			LapNumber:    e.LapNumber,
			DriverNumber: e.DriverNumber,
			DriverName:   driverName,
			Message:      e.Message,
			Significant:  sig,
		})
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date < events[j].Date
	})

	name := m.MeetingName
	if name == "" {
		name = s.Location + " Grand Prix"
	}
	now := time.Now().UTC()
	current := toF1Session(s, m.IsCancelled, now)

	// Weekend sessions for session-type pills.
	var weekend []session
	wq := url.Values{}
	wq.Set("meeting_key", strconv.Itoa(s.MeetingKey))
	_ = c.getJSON(ctx, "/sessions", wq, &weekend)
	sort.Slice(weekend, func(i, j int) bool {
		return weekend[i].DateStart < weekend[j].DateStart
	})
	weekendRefs := make([]domain.F1Session, 0, len(weekend))
	for _, ws := range weekend {
		ref := toF1Session(ws, m.IsCancelled, now)
		if ref.Kind == domain.F1KindOther {
			continue
		}
		weekendRefs = append(weekendRefs, ref)
	}

	race := domain.F1Race{
		MeetingKey:       s.MeetingKey,
		SessionKey:       s.SessionKey,
		Year:             s.Year,
		Name:             name,
		OfficialName:     m.MeetingOfficialName,
		Location:         firstNonEmpty(s.Location, m.Location),
		CountryName:      firstNonEmpty(s.CountryName, m.CountryName),
		CountryCode:      firstNonEmpty(s.CountryCode, m.CountryCode),
		CircuitShortName: firstNonEmpty(s.CircuitShortName, m.CircuitShortName),
		DateStart:        s.DateStart,
		DateEnd:          s.DateEnd,
		Status:           current.Status,
		Sessions:         weekendRefs,
	}

	return &domain.F1RaceDetail{
		Race:     race,
		Session:  current,
		Results:  results,
		Events:   events,
		Sessions: weekendRefs,
	}, nil
}

func (c *Client) Standings(ctx context.Context, year int) (*domain.F1Standings, error) {
	if year <= 0 {
		year = time.Now().UTC().Year()
	}
	races, err := c.ListRaces(ctx, year)
	if err != nil {
		return nil, err
	}
	sessionKey := 0
	meetingName := ""
	for _, r := range races {
		if r.Status == domain.F1Completed || r.Status == domain.F1InProgress {
			sessionKey = r.SessionKey
			meetingName = r.Name
			break
		}
	}
	if sessionKey == 0 {
		// Fall back to OpenF1 "latest" for the current season when no completed race yet.
		if year == time.Now().UTC().Year() {
			return c.standingsForSession(ctx, year, 0, "latest", "")
		}
		if len(races) == 0 {
			return &domain.F1Standings{Year: year, Drivers: []domain.F1DriverStanding{}, Constructors: []domain.F1TeamStanding{}}, nil
		}
		sessionKey = races[len(races)-1].SessionKey
		meetingName = races[len(races)-1].Name
	}
	return c.standingsForSession(ctx, year, sessionKey, strconv.Itoa(sessionKey), meetingName)
}

func (c *Client) standingsForSession(ctx context.Context, year, sessionKey int, sessionParam, meetingName string) (*domain.F1Standings, error) {
	q := url.Values{}
	q.Set("session_key", sessionParam)

	driversByNum := map[int]struct{ Name, Acronym, Team string }{}
	var drivers []struct {
		DriverNumber int    `json:"driver_number"`
		FullName     string `json:"full_name"`
		NameAcronym  string `json:"name_acronym"`
		TeamName     string `json:"team_name"`
	}
	_ = c.getJSON(ctx, "/drivers", q, &drivers)
	for _, d := range drivers {
		driversByNum[d.DriverNumber] = struct{ Name, Acronym, Team string }{
			Name: d.FullName, Acronym: d.NameAcronym, Team: d.TeamName,
		}
	}

	var rawDrivers []struct {
		DriverNumber    int     `json:"driver_number"`
		MeetingKey      int     `json:"meeting_key"`
		SessionKey      int     `json:"session_key"`
		PositionCurrent int     `json:"position_current"`
		PointsCurrent   float64 `json:"points_current"`
	}
	if err := c.getJSON(ctx, "/championship_drivers", q, &rawDrivers); err != nil {
		return nil, err
	}
	wdc := make([]domain.F1DriverStanding, 0, len(rawDrivers))
	for _, r := range rawDrivers {
		info := driversByNum[r.DriverNumber]
		name := info.Name
		if name == "" {
			name = fmt.Sprintf("Driver #%d", r.DriverNumber)
		}
		wdc = append(wdc, domain.F1DriverStanding{
			Position:     r.PositionCurrent,
			DriverNumber: r.DriverNumber,
			Name:         name,
			NameAcronym:  info.Acronym,
			TeamName:     info.Team,
			Points:       r.PointsCurrent,
		})
		if sessionKey == 0 && r.SessionKey != 0 {
			sessionKey = r.SessionKey
		}
	}
	sort.Slice(wdc, func(i, j int) bool {
		if wdc[i].Position == wdc[j].Position {
			return wdc[i].Points > wdc[j].Points
		}
		return wdc[i].Position < wdc[j].Position
	})

	var rawTeams []struct {
		TeamName        string  `json:"team_name"`
		SessionKey      int     `json:"session_key"`
		PositionCurrent int     `json:"position_current"`
		PointsCurrent   float64 `json:"points_current"`
	}
	if err := c.getJSON(ctx, "/championship_teams", q, &rawTeams); err != nil {
		return nil, err
	}
	wcc := make([]domain.F1TeamStanding, 0, len(rawTeams))
	for _, r := range rawTeams {
		wcc = append(wcc, domain.F1TeamStanding{
			Position: r.PositionCurrent,
			TeamName: r.TeamName,
			Points:   r.PointsCurrent,
		})
		if sessionKey == 0 && r.SessionKey != 0 {
			sessionKey = r.SessionKey
		}
	}
	sort.Slice(wcc, func(i, j int) bool {
		if wcc[i].Position == wcc[j].Position {
			return wcc[i].Points > wcc[j].Points
		}
		return wcc[i].Position < wcc[j].Position
	})

	return &domain.F1Standings{
		Year:         year,
		SessionKey:   sessionKey,
		MeetingName:  meetingName,
		Drivers:      wdc,
		Constructors: wcc,
	}, nil
}

func isSignificantEvent(category, flag, message string) bool {
	cat := strings.ToLower(category)
	msg := strings.ToUpper(message)
	switch cat {
	case "flag":
		f := strings.ToUpper(flag)
		return f != "" && f != "GREEN" && f != "CLEAR"
	case "safetycar", "safety car", "incident", "sessionstatus":
		return true
	default:
		return strings.Contains(msg, "SAFETY CAR") ||
			strings.Contains(msg, "VIRTUAL SAFETY") ||
			strings.Contains(msg, "RED FLAG") ||
			strings.Contains(msg, "SESSION") && (strings.Contains(msg, "STARTED") || strings.Contains(msg, "FINISHED") || strings.Contains(msg, "ABORTED"))
	}
}

func formatGap(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case float64:
		if t == 0 {
			return "—"
		}
		return fmt.Sprintf("+%.3f", t)
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
