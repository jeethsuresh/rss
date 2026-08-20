package domain

// Dota 2 esports models (PandaScore events/matches; STRATZ game detail). Distinct from MLB/F1.

type DotaEventTier string

const (
	DotaTierPremier      DotaEventTier = "premier"
	DotaTierProfessional DotaEventTier = "professional"
	DotaTierSemiPro      DotaEventTier = "semi-pro"
	DotaTierAmateur      DotaEventTier = "amateur"
	DotaTierUnknown      DotaEventTier = "unknown"
)

type DotaEventStatus string

const (
	DotaEventUpcoming  DotaEventStatus = "upcoming"
	DotaEventOngoing   DotaEventStatus = "ongoing"
	DotaEventCompleted DotaEventStatus = "completed"
)

type DotaMatchStatus string

const (
	DotaMatchUpcoming  DotaMatchStatus = "upcoming"
	DotaMatchLive      DotaMatchStatus = "live"
	DotaMatchCompleted DotaMatchStatus = "completed"
	DotaMatchCanceled  DotaMatchStatus = "canceled"
)

type DotaEventType string

const (
	DotaEventLeague     DotaEventType = "league"
	DotaEventTournament DotaEventType = "tournament"
)

type DotaSide string

const (
	DotaRadiant DotaSide = "radiant"
	DotaDire    DotaSide = "dire"
)

type DotaSeason struct {
	Year int `json:"year"`
}

type DotaTeam struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName,omitempty"`
	LogoURL   string `json:"logoUrl,omitempty"`
}

type DotaEvent struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         DotaEventType   `json:"type"`
	Tier         DotaEventTier   `json:"tier"`
	Status       DotaEventStatus `json:"status"`
	StartAt      *string         `json:"startAt,omitempty"`
	EndAt        *string         `json:"endAt,omitempty"`
	LogoURL      string          `json:"logoUrl,omitempty"`
	LeagueID     string          `json:"leagueId,omitempty"`
	LeagueName   string          `json:"leagueName,omitempty"`
	TournamentID string          `json:"tournamentId,omitempty"`
	Year         int             `json:"year"`
	Organizer    string          `json:"organizer,omitempty"`
}

type DotaPinnedEvent struct {
	EventID   string        `json:"eventId"`
	EventType DotaEventType `json:"eventType"`
}

type DotaMatch struct {
	ID          int             `json:"id"`
	EventID     string          `json:"eventId,omitempty"`
	EventName   string          `json:"eventName,omitempty"`
	TeamA       DotaTeam        `json:"teamA"`
	TeamB       DotaTeam        `json:"teamB"`
	ScheduledAt *string         `json:"scheduledAt,omitempty"`
	Status      DotaMatchStatus `json:"status"`
	BestOf      *int            `json:"bestOf,omitempty"`
	ScoreA      *int            `json:"scoreA,omitempty"`
	ScoreB      *int            `json:"scoreB,omitempty"`
	Year        int             `json:"year"`
	Stage       string          `json:"stage,omitempty"`
}

type DotaHeroPick struct {
	HeroID   int      `json:"heroId"`
	HeroName string   `json:"heroName"`
	PlayerID *int64   `json:"playerId,omitempty"`
	Team     DotaSide `json:"team,omitempty"`
}

type DotaHeroBan struct {
	HeroID   int      `json:"heroId"`
	HeroName string   `json:"heroName"`
	Team     DotaSide `json:"team,omitempty"`
}

type DotaItem struct {
	ItemID int    `json:"itemId"`
	Name   string `json:"name,omitempty"`
}

type DotaPlayer struct {
	PlayerID          int64      `json:"playerId"`
	Name              string     `json:"name"`
	HeroID            int        `json:"heroId"`
	HeroName          string     `json:"heroName"`
	Team              DotaSide   `json:"team"`
	Kills             int        `json:"kills"`
	Deaths            int        `json:"deaths"`
	Assists           int        `json:"assists"`
	GPM               int        `json:"gpm"`
	XPM               int        `json:"xpm"`
	NetWorth          int        `json:"netWorth"`
	Items             []DotaItem `json:"items,omitempty"`
}

type DotaGame struct {
	ID              string         `json:"id"`
	MatchID         int            `json:"matchId"`
	GameIndex       int            `json:"gameIndex"`
	StartedAt       *string        `json:"startedAt,omitempty"`
	DurationSeconds *int           `json:"durationSeconds,omitempty"`
	Winner          DotaSide       `json:"winner,omitempty"`
	WinnerTeamName  string         `json:"winnerTeamName,omitempty"`
	RadiantTeam     *DotaTeam      `json:"radiantTeam,omitempty"`
	DireTeam        *DotaTeam      `json:"direTeam,omitempty"`
	RadiantScore    *int           `json:"radiantScore,omitempty"`
	DireScore       *int           `json:"direScore,omitempty"`
	Heroes          []DotaHeroPick `json:"heroes,omitempty"`
	Bans            []DotaHeroBan  `json:"bans,omitempty"`
	Players         []DotaPlayer   `json:"players,omitempty"`
	StratzMatchID   *int64         `json:"stratzMatchId,omitempty"`
	MappingConfidence string       `json:"mappingConfidence,omitempty"` // exact | inferred | unknown
	DetailAvailable bool           `json:"detailAvailable"`
	DetailError     string         `json:"detailError,omitempty"`
}

type DotaMatchDetail struct {
	Match  DotaMatch  `json:"match"`
	Games  []DotaGame `json:"games"`
	Live   bool       `json:"live"`
}

type DotaProvidersStatus struct {
	Provider             string `json:"provider"` // opendota | pandascore
	Ready                bool   `json:"ready"`
	PandaScoreConfigured bool   `json:"pandaScoreConfigured"`
	StratzConfigured     bool   `json:"stratzConfigured"`
}
