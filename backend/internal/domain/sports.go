package domain

import (
	"context"
	"time"
)

// MLB / Sports normalized models (never expose raw Stats API JSON to the UI).

type MlbGameStatus string

const (
	MlbScheduled MlbGameStatus = "scheduled"
	MlbPreGame   MlbGameStatus = "pre_game"
	MlbLive      MlbGameStatus = "live"
	MlbFinal     MlbGameStatus = "final"
	MlbPostponed MlbGameStatus = "postponed"
	MlbCancelled MlbGameStatus = "cancelled"
	MlbUnknown   MlbGameStatus = "unknown"
)

type MlbTeam struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	ShortName    string `json:"shortName,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
}

type MlbSeason struct {
	SeasonID              int    `json:"seasonId"`
	RegularSeasonStartDate string `json:"regularSeasonStartDate,omitempty"`
	RegularSeasonEndDate   string `json:"regularSeasonEndDate,omitempty"`
}

type MlbGame struct {
	ID                int           `json:"id"` // gamePk
	Season            int           `json:"season"`
	GameDate          string        `json:"gameDate"`
	OfficialDate      string        `json:"officialDate,omitempty"`
	Status            MlbGameStatus `json:"status"`
	StatusDetail      string        `json:"statusDetail,omitempty"`
	AwayTeam          MlbTeam       `json:"awayTeam"`
	HomeTeam          MlbTeam       `json:"homeTeam"`
	AwayScore         *int          `json:"awayScore,omitempty"`
	HomeScore         *int          `json:"homeScore,omitempty"`
	CurrentInning     *int          `json:"currentInning,omitempty"`
	CurrentInningHalf string        `json:"currentInningHalf,omitempty"` // top | bottom
}

type MlbInning struct {
	Number     int `json:"number"`
	AwayRuns   int `json:"awayRuns"`
	HomeRuns   int `json:"homeRuns"`
	AwayHits   int `json:"awayHits,omitempty"`
	HomeHits   int `json:"homeHits,omitempty"`
	AwayErrors int `json:"awayErrors,omitempty"`
	HomeErrors int `json:"homeErrors,omitempty"`
}

type MlbPlay struct {
	ID            string `json:"id"`
	Inning        int    `json:"inning"`
	Half          string `json:"half"` // top | bottom
	Event         string `json:"event"`
	Description   string `json:"description"`
	IsScoringPlay bool   `json:"isScoringPlay"`
	AwayScore     *int   `json:"awayScore,omitempty"`
	HomeScore     *int   `json:"homeScore,omitempty"`
	AtBatIndex    *int   `json:"atBatIndex,omitempty"`
}

type MlbBatterLine struct {
	PlayerID   int    `json:"playerId"`
	Name       string `json:"name"`
	Position   string `json:"position,omitempty"`
	BattingOrder int  `json:"battingOrder,omitempty"`
	AtBats     int    `json:"atBats"`
	Runs       int    `json:"runs"`
	Hits       int    `json:"hits"`
	RBI        int    `json:"rbi"`
	Walks      int    `json:"walks"`
	StrikeOuts int    `json:"strikeOuts"`
	HomeRuns   int    `json:"homeRuns"`
	Summary    string `json:"summary,omitempty"`
}

type MlbPitcherLine struct {
	PlayerID       int    `json:"playerId"`
	Name           string `json:"name"`
	Note           string `json:"note,omitempty"`
	InningsPitched string `json:"inningsPitched"`
	Hits           int    `json:"hits"`
	Runs           int    `json:"runs"`
	EarnedRuns     int    `json:"earnedRuns"`
	Walks          int    `json:"walks"`
	StrikeOuts     int    `json:"strikeOuts"`
	HomeRuns       int    `json:"homeRuns"`
	PitchesThrown  int    `json:"pitchesThrown"`
	Strikes        int    `json:"strikes,omitempty"`
	Summary        string `json:"summary,omitempty"`
}

type MlbTeamBox struct {
	Team     MlbTeam          `json:"team"`
	Batters  []MlbBatterLine  `json:"batters"`
	Pitchers []MlbPitcherLine `json:"pitchers"`
}

type MlbGameDetail struct {
	Game       MlbGame     `json:"game"`
	Innings    []MlbInning `json:"innings"`
	Plays      []MlbPlay   `json:"plays"`
	AwayHits   int         `json:"awayHits,omitempty"`
	HomeHits   int         `json:"homeHits,omitempty"`
	AwayErrors int         `json:"awayErrors,omitempty"`
	HomeErrors int         `json:"homeErrors,omitempty"`
	AwayBox    *MlbTeamBox `json:"awayBox,omitempty"`
	HomeBox    *MlbTeamBox `json:"homeBox,omitempty"`
}

type SportsRepository interface {
	GetFollowedTeamIDs(ctx context.Context) ([]int, error)
	SetFollowedTeamIDs(ctx context.Context, ids []int) error
	GetDotaFollowedTeamIDs(ctx context.Context, provider string) ([]int, error)
	SetDotaFollowedTeamIDs(ctx context.Context, provider string, ids []int) error
	GetDotaPinnedEvents(ctx context.Context, provider string) ([]DotaPinnedEvent, error)
	SetDotaPinnedEvents(ctx context.Context, provider string, pins []DotaPinnedEvent) error
}

type SportsCacheRepository interface {
	Get(ctx context.Context, key string) (payload []byte, fetchedAt time.Time, ok bool, err error)
	Set(ctx context.Context, key string, payload []byte) error
	Delete(ctx context.Context, key string) error
}
