package domain

// MLB league standings (divisions + wild card).

type MlbStandingRow struct {
	Rank               int     `json:"rank"`
	Team               MlbTeam `json:"team"`
	Wins               int     `json:"wins"`
	Losses             int     `json:"losses"`
	WinningPercentage  string  `json:"winningPercentage"`
	GamesBack          string  `json:"gamesBack"`
	WildCardGamesBack  string  `json:"wildCardGamesBack,omitempty"`
	RunDifferential    int     `json:"runDifferential"`
	Streak             string  `json:"streak,omitempty"`
	DivisionLeader     bool    `json:"divisionLeader,omitempty"`
	Clinched           bool    `json:"clinched,omitempty"`
}

type MlbStandingSection struct {
	ID       string           `json:"id"`
	League   string           `json:"league"`   // AL | NL
	Name     string           `json:"name"`     // e.g. East, Wild Card
	Kind     string           `json:"kind"`     // division | wildcard
	Teams    []MlbStandingRow `json:"teams"`
}

type MlbStandings struct {
	Season   int                   `json:"season"`
	Sections []MlbStandingSection  `json:"sections"`
}
