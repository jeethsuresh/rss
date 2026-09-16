package openf1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

const jolpicaKeyBase = 1_000_000_000

const (
	jolpicaKindRace        = 0
	jolpicaKindQuali       = 1
	jolpicaKindSprint      = 2
	jolpicaKindSprintQuali = 3
	jolpicaKindFP1         = 4
	jolpicaKindFP2         = 5
	jolpicaKindFP3         = 6
)

func jolpicaMeetingKey(year, round int) int {
	return jolpicaKeyBase + year*1000 + round
}

func jolpicaSessionKey(year, round, kind int) int {
	return jolpicaKeyBase + year*10000 + round*10 + kind
}

func decodeJolpicaSession(key int) (year, round, kind int, ok bool) {
	if key < jolpicaKeyBase {
		return 0, 0, 0, false
	}
	n := key - jolpicaKeyBase
	kind = n % 10
	n /= 10
	round = n % 1000
	year = n / 1000
	return year, round, kind, year >= 2023 && round > 0
}

func (c *Client) jolpicaBase() string {
	return jolpicaBaseURL
}

func (c *Client) getJolpicaJSON(ctx context.Context, path string, dest any) error {
	u := strings.TrimRight(c.jolpicaBase(), "/") + path
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
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: jolpica status %d", domain.ErrNetwork, res.StatusCode)
	}
	return json.Unmarshal(body, dest)
}

type jolpicaMRData struct {
	MRData struct {
		Total       string `json:"total"`
		SeasonTable struct {
			Seasons []struct {
				Season string `json:"season"`
			} `json:"Seasons"`
		} `json:"SeasonTable"`
		RaceTable struct {
			Races []jolpicaRace `json:"Races"`
		} `json:"RaceTable"`
		StandingsTable struct {
			StandingsLists []jolpicaStandingsList `json:"StandingsLists"`
		} `json:"StandingsTable"`
	} `json:"MRData"`
}

type jolpicaRace struct {
	Season            string               `json:"season"`
	Round             string               `json:"round"`
	RaceName          string               `json:"raceName"`
	Date              string               `json:"date"`
	Time              string               `json:"time"`
	Circuit           jolpicaCircuit       `json:"Circuit"`
	FirstPractice     *jolpicaSessionTime  `json:"FirstPractice"`
	SecondPractice    *jolpicaSessionTime  `json:"SecondPractice"`
	ThirdPractice     *jolpicaSessionTime  `json:"ThirdPractice"`
	Qualifying        *jolpicaSessionTime  `json:"Qualifying"`
	Sprint            *jolpicaSessionTime  `json:"Sprint"`
	SprintQualifying  *jolpicaSessionTime  `json:"SprintQualifying"`
	Results           []jolpicaResult      `json:"Results"`
	QualifyingResults []jolpicaQualiResult `json:"QualifyingResults"`
	SprintResults     []jolpicaResult      `json:"SprintResults"`
}

type jolpicaCircuit struct {
	CircuitName string `json:"circuitName"`
	Location    struct {
		Locality string `json:"locality"`
		Country  string `json:"country"`
	} `json:"Location"`
}

type jolpicaSessionTime struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

type jolpicaDriver struct {
	Code            string `json:"code"`
	GivenName       string `json:"givenName"`
	FamilyName      string `json:"familyName"`
	PermanentNumber string `json:"permanentNumber"`
}

type jolpicaResult struct {
	Number       string        `json:"number"`
	Position     string        `json:"position"`
	PositionText string        `json:"positionText"`
	Points       string        `json:"points"`
	Laps         string        `json:"laps"`
	Status       string        `json:"status"`
	Driver       jolpicaDriver `json:"Driver"`
	Constructor  struct {
		Name string `json:"name"`
	} `json:"Constructor"`
	Time *struct {
		Time string `json:"time"`
	} `json:"Time"`
}

type jolpicaQualiResult struct {
	Number      string        `json:"number"`
	Position    string        `json:"position"`
	Driver      jolpicaDriver `json:"Driver"`
	Constructor struct {
		Name string `json:"name"`
	} `json:"Constructor"`
	Q1 string `json:"Q1"`
	Q2 string `json:"Q2"`
	Q3 string `json:"Q3"`
}

type jolpicaStandingsList struct {
	Round                string                  `json:"round"`
	DriverStandings      []jolpicaDriverStanding `json:"DriverStandings"`
	ConstructorStandings []jolpicaTeamStanding   `json:"ConstructorStandings"`
}

type jolpicaDriverStanding struct {
	Position     string        `json:"position"`
	Points       string        `json:"points"`
	Driver       jolpicaDriver `json:"Driver"`
	Constructors []struct {
		Name string `json:"name"`
	} `json:"Constructors"`
}

type jolpicaTeamStanding struct {
	Position    string `json:"position"`
	Points      string `json:"points"`
	Constructor struct {
		Name string `json:"name"`
	} `json:"Constructor"`
}

func (c *Client) listYearsJolpica(ctx context.Context) ([]int, error) {
	var raw jolpicaMRData
	if err := c.getJolpicaJSON(ctx, "/seasons.json?limit=100", &raw); err != nil {
		return nil, err
	}
	years := make([]int, 0, 8)
	for _, s := range raw.MRData.SeasonTable.Seasons {
		y, _ := strconv.Atoi(s.Season)
		if y >= 2023 {
			years = append(years, y)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	return years, nil
}

func (c *Client) listRacesJolpica(ctx context.Context, year int) ([]domain.F1Race, error) {
	var raw jolpicaMRData
	if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/races.json", year), &raw); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]domain.F1Race, 0, len(raw.MRData.RaceTable.Races))
	for _, jr := range raw.MRData.RaceTable.Races {
		race, ok := jolpicaRaceToDomain(jr, now)
		if !ok {
			continue
		}
		out = append(out, race)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DateStart > out[j].DateStart
	})
	return out, nil
}

func jolpicaRaceToDomain(jr jolpicaRace, now time.Time) (domain.F1Race, bool) {
	year, _ := strconv.Atoi(jr.Season)
	round, _ := strconv.Atoi(jr.Round)
	if year <= 0 || round <= 0 {
		return domain.F1Race{}, false
	}
	loc := jr.Circuit.Location.Locality
	country := jr.Circuit.Location.Country
	sessions := make([]domain.F1Session, 0, 6)
	add := func(st *jolpicaSessionTime, name string, kind domain.F1SessionKind, k int) {
		if st == nil {
			return
		}
		start := parseJolpicaTime(st.Date, st.Time)
		if start == "" {
			return
		}
		end := sessionEndRFC(start, kind)
		sessions = append(sessions, domain.F1Session{
			SessionKey:  jolpicaSessionKey(year, round, k),
			SessionName: name,
			Kind:        kind,
			DateStart:   start,
			DateEnd:     end,
			Status:      raceStatus(start, end, false, now),
		})
	}
	add(jr.FirstPractice, "Practice 1", domain.F1KindPractice, jolpicaKindFP1)
	add(jr.SecondPractice, "Practice 2", domain.F1KindPractice, jolpicaKindFP2)
	add(jr.ThirdPractice, "Practice 3", domain.F1KindPractice, jolpicaKindFP3)
	add(jr.SprintQualifying, "Sprint Qualifying", domain.F1KindSprintQuali, jolpicaKindSprintQuali)
	add(jr.Qualifying, "Qualifying", domain.F1KindQuali, jolpicaKindQuali)
	add(jr.Sprint, "Sprint", domain.F1KindSprint, jolpicaKindSprint)
	raceStart := parseJolpicaTime(jr.Date, jr.Time)
	if raceStart == "" {
		return domain.F1Race{}, false
	}
	raceEnd := sessionEndRFC(raceStart, domain.F1KindRace)
	raceSess := domain.F1Session{
		SessionKey:  jolpicaSessionKey(year, round, jolpicaKindRace),
		SessionName: "Race",
		Kind:        domain.F1KindRace,
		DateStart:   raceStart,
		DateEnd:     raceEnd,
		Status:      raceStatus(raceStart, raceEnd, false, now),
	}
	sessions = append(sessions, raceSess)
	return domain.F1Race{
		MeetingKey:       jolpicaMeetingKey(year, round),
		SessionKey:       raceSess.SessionKey,
		Year:             year,
		Name:             jr.RaceName,
		OfficialName:     jr.RaceName,
		Location:         loc,
		CountryName:      country,
		CountryCode:      countryCodeFor(country),
		CircuitShortName: firstNonEmpty(jr.Circuit.CircuitName, loc),
		DateStart:        raceStart,
		DateEnd:          raceEnd,
		Status:           raceSess.Status,
		Sessions:         sessions,
	}, true
}

func (c *Client) raceDetailJolpica(ctx context.Context, year, round, kind int) (*domain.F1RaceDetail, error) {
	races, err := c.listRacesJolpica(ctx, year)
	if err != nil {
		return nil, err
	}
	var race domain.F1Race
	foundMeeting := false
	for _, r := range races {
		if r.MeetingKey == jolpicaMeetingKey(year, round) {
			race = r
			foundMeeting = true
			break
		}
	}
	if !foundMeeting {
		return nil, domain.ErrNotFound
	}
	current := race.Sessions[len(race.Sessions)-1]
	for _, s := range race.Sessions {
		if s.SessionKey == jolpicaSessionKey(year, round, kind) {
			current = s
			break
		}
	}
	race.SessionKey = current.SessionKey
	race.DateStart = current.DateStart
	race.DateEnd = current.DateEnd
	race.Status = current.Status

	results, _ := c.jolpicaResults(ctx, year, round, kind)
	return &domain.F1RaceDetail{
		Race:     race,
		Session:  current,
		Results:  results,
		Events:   []domain.F1Event{},
		Sessions: race.Sessions,
	}, nil
}

func (c *Client) RaceDetailFromRace(ctx context.Context, race domain.F1Race) (*domain.F1RaceDetail, error) {
	if year, round, kind, ok := decodeJolpicaSession(race.SessionKey); ok {
		return c.raceDetailJolpica(ctx, year, round, kind)
	}
	if race.Year <= 0 {
		return nil, domain.ErrInvalidParams
	}
	races, err := c.listRacesJolpica(ctx, race.Year)
	if err != nil {
		return nil, err
	}
	kind := jolpicaKindRace
	for _, sess := range race.Sessions {
		if sess.SessionKey == race.SessionKey {
			kind = kindFromSessionName(sess.SessionName)
			break
		}
	}
	for _, candidate := range races {
		if !sameF1Meeting(candidate, race) {
			continue
		}
		_, round, _, ok := decodeJolpicaSession(candidate.SessionKey)
		if !ok {
			continue
		}
		return c.raceDetailJolpica(ctx, race.Year, round, kind)
	}
	return nil, domain.ErrNotFound
}

func sameF1Meeting(a, b domain.F1Race) bool {
	if a.Year != 0 && b.Year != 0 && a.Year != b.Year {
		return false
	}
	if a.Name != "" && b.Name != "" && strings.EqualFold(a.Name, b.Name) {
		return true
	}
	if len(a.DateStart) >= 10 && len(b.DateStart) >= 10 && a.DateStart[:10] == b.DateStart[:10] {
		return true
	}
	return a.CircuitShortName != "" && strings.EqualFold(a.CircuitShortName, b.CircuitShortName)
}

func kindFromSessionName(name string) int {
	switch sessionKind(name) {
	case domain.F1KindQuali:
		return jolpicaKindQuali
	case domain.F1KindSprint:
		return jolpicaKindSprint
	case domain.F1KindSprintQuali:
		return jolpicaKindSprintQuali
	case domain.F1KindPractice:
		switch name {
		case "Practice 1":
			return jolpicaKindFP1
		case "Practice 2":
			return jolpicaKindFP2
		case "Practice 3":
			return jolpicaKindFP3
		default:
			return jolpicaKindFP1
		}
	default:
		return jolpicaKindRace
	}
}

func (c *Client) jolpicaResults(ctx context.Context, year, round, kind int) ([]domain.F1DriverResult, error) {
	switch kind {
	case jolpicaKindQuali, jolpicaKindSprintQuali:
		var raw jolpicaMRData
		if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/%d/qualifying.json", year, round), &raw); err != nil {
			return nil, err
		}
		if len(raw.MRData.RaceTable.Races) == 0 {
			return []domain.F1DriverResult{}, nil
		}
		out := make([]domain.F1DriverResult, 0, len(raw.MRData.RaceTable.Races[0].QualifyingResults))
		for _, r := range raw.MRData.RaceTable.Races[0].QualifyingResults {
			pos, _ := strconv.Atoi(r.Position)
			out = append(out, domain.F1DriverResult{
				Position:     pos,
				DriverNumber: driverNumber(r.Number, r.Driver.PermanentNumber),
				Name:         strings.TrimSpace(r.Driver.GivenName + " " + r.Driver.FamilyName),
				NameAcronym:  r.Driver.Code,
				TeamName:     r.Constructor.Name,
				GapToLeader:  firstNonEmpty(r.Q3, r.Q2, r.Q1),
			})
		}
		return out, nil
	case jolpicaKindSprint:
		var raw jolpicaMRData
		if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/%d/sprint.json", year, round), &raw); err != nil {
			return nil, err
		}
		if len(raw.MRData.RaceTable.Races) == 0 {
			return []domain.F1DriverResult{}, nil
		}
		return mapJolpicaResults(raw.MRData.RaceTable.Races[0].SprintResults), nil
	default:
		var raw jolpicaMRData
		if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/%d/results.json", year, round), &raw); err != nil {
			return nil, err
		}
		if len(raw.MRData.RaceTable.Races) == 0 {
			return []domain.F1DriverResult{}, nil
		}
		return mapJolpicaResults(raw.MRData.RaceTable.Races[0].Results), nil
	}
}

func mapJolpicaResults(rows []jolpicaResult) []domain.F1DriverResult {
	out := make([]domain.F1DriverResult, 0, len(rows))
	for _, r := range rows {
		pos, _ := strconv.Atoi(r.Position)
		pts, _ := strconv.ParseFloat(r.Points, 64)
		laps, _ := strconv.Atoi(r.Laps)
		status := strings.ToLower(r.Status)
		gap := ""
		if r.Time != nil {
			gap = r.Time.Time
		}
		if pos == 1 {
			gap = "—"
		}
		out = append(out, domain.F1DriverResult{
			Position:     pos,
			DriverNumber: driverNumber(r.Number, r.Driver.PermanentNumber),
			Name:         strings.TrimSpace(r.Driver.GivenName + " " + r.Driver.FamilyName),
			NameAcronym:  r.Driver.Code,
			TeamName:     r.Constructor.Name,
			Points:       pts,
			Laps:         laps,
			DNF:          strings.Contains(status, "retired") || strings.Contains(status, "accident") || strings.Contains(status, "collision"),
			DNS:          strings.Contains(status, "did not start") || r.PositionText == "DNS",
			DSQ:          strings.Contains(status, "disqual") || r.PositionText == "DSQ" || r.PositionText == "DQ",
			GapToLeader:  gap,
		})
	}
	return out
}

func (c *Client) standingsJolpica(ctx context.Context, year int, races []domain.F1Race) (*domain.F1Standings, error) {
	var driversRaw jolpicaMRData
	if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/driverStandings.json", year), &driversRaw); err != nil {
		return nil, err
	}
	var teamsRaw jolpicaMRData
	if err := c.getJolpicaJSON(ctx, fmt.Sprintf("/%d/constructorStandings.json", year), &teamsRaw); err != nil {
		return nil, err
	}
	wdc := []domain.F1DriverStanding{}
	if lists := driversRaw.MRData.StandingsTable.StandingsLists; len(lists) > 0 {
		for _, r := range lists[0].DriverStandings {
			pos, _ := strconv.Atoi(r.Position)
			pts, _ := strconv.ParseFloat(r.Points, 64)
			team := ""
			if len(r.Constructors) > 0 {
				team = r.Constructors[0].Name
			}
			wdc = append(wdc, domain.F1DriverStanding{
				Position:     pos,
				DriverNumber: driverNumber("", r.Driver.PermanentNumber),
				Name:         strings.TrimSpace(r.Driver.GivenName + " " + r.Driver.FamilyName),
				NameAcronym:  r.Driver.Code,
				TeamName:     team,
				Points:       pts,
			})
		}
	}
	wcc := []domain.F1TeamStanding{}
	if lists := teamsRaw.MRData.StandingsTable.StandingsLists; len(lists) > 0 {
		for _, r := range lists[0].ConstructorStandings {
			pos, _ := strconv.Atoi(r.Position)
			pts, _ := strconv.ParseFloat(r.Points, 64)
			wcc = append(wcc, domain.F1TeamStanding{
				Position: pos,
				TeamName: r.Constructor.Name,
				Points:   pts,
			})
		}
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
	return &domain.F1Standings{
		Year:         year,
		SessionKey:   sessionKey,
		MeetingName:  meetingName,
		Drivers:      wdc,
		Constructors: wcc,
	}, nil
}

func driverNumber(number, permanent string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(number)); err == nil {
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(permanent))
	return n
}

func parseJolpicaTime(date, clock string) string {
	date = strings.TrimSpace(date)
	clock = strings.TrimSpace(clock)
	if date == "" {
		return ""
	}
	if clock == "" {
		clock = "00:00:00Z"
	}
	if !strings.Contains(clock, "Z") && !strings.ContainsAny(clock, "+-") {
		clock += "Z"
	}
	stamp := date + "T" + clock
	if _, err := time.Parse(time.RFC3339, stamp); err != nil {
		if t, err2 := time.Parse("2006-01-02T15:04:05Z07:00", stamp); err2 == nil {
			return t.UTC().Format(time.RFC3339)
		}
		return date + "T00:00:00Z"
	}
	return stamp
}

func sessionEndRFC(start string, kind domain.F1SessionKind) string {
	st, err := time.Parse(time.RFC3339, start)
	if err != nil {
		st, err = time.Parse("2006-01-02T15:04:05Z07:00", start)
	}
	if err != nil {
		return start
	}
	d := time.Hour
	if kind == domain.F1KindRace {
		d = 2 * time.Hour
	}
	return st.Add(d).UTC().Format(time.RFC3339)
}

func countryCodeFor(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "australia":
		return "AUS"
	case "austria":
		return "AUT"
	case "azerbaijan":
		return "AZE"
	case "bahrain":
		return "BHR"
	case "belgium":
		return "BEL"
	case "brazil":
		return "BRA"
	case "canada":
		return "CAN"
	case "china":
		return "CHN"
	case "france":
		return "FRA"
	case "germany":
		return "DEU"
	case "hungary":
		return "HUN"
	case "italy":
		return "ITA"
	case "japan":
		return "JPN"
	case "malaysia":
		return "MYS"
	case "mexico":
		return "MEX"
	case "monaco":
		return "MON"
	case "netherlands":
		return "NED"
	case "portugal":
		return "POR"
	case "qatar":
		return "QAT"
	case "saudi arabia":
		return "KSA"
	case "singapore":
		return "SGP"
	case "spain":
		return "ESP"
	case "uae", "united arab emirates":
		return "UAE"
	case "uk", "united kingdom", "great britain":
		return "GBR"
	case "usa", "united states", "united states of america":
		return "USA"
	default:
		if len(name) >= 3 {
			return strings.ToUpper(name[:3])
		}
		return strings.ToUpper(name)
	}
}
