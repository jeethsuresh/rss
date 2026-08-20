package domain

// Normalized OpenF1 models (UI never sees raw OpenF1 JSON).

type F1RaceStatus string

const (
	F1Scheduled  F1RaceStatus = "scheduled"
	F1InProgress F1RaceStatus = "in_progress"
	F1Completed  F1RaceStatus = "completed"
	F1Cancelled  F1RaceStatus = "cancelled"
)

type F1Season struct {
	Year int `json:"year"`
}

type F1Race struct {
	MeetingKey       int          `json:"meetingKey"`
	SessionKey       int          `json:"sessionKey"`
	Year             int          `json:"year"`
	Name             string       `json:"name"`
	OfficialName     string       `json:"officialName,omitempty"`
	Location         string       `json:"location"`
	CountryName      string       `json:"countryName"`
	CountryCode      string       `json:"countryCode,omitempty"`
	CircuitShortName string       `json:"circuitShortName"`
	DateStart        string       `json:"dateStart"`
	DateEnd          string       `json:"dateEnd"`
	Status           F1RaceStatus `json:"status"`
}

type F1DriverResult struct {
	Position     int     `json:"position"`
	DriverNumber int     `json:"driverNumber"`
	Name         string  `json:"name"`
	NameAcronym  string  `json:"nameAcronym,omitempty"`
	TeamName     string  `json:"teamName,omitempty"`
	Points       float64 `json:"points"`
	Laps         int     `json:"laps"`
	DNF          bool    `json:"dnf"`
	DNS          bool    `json:"dns"`
	DSQ          bool    `json:"dsq"`
	GapToLeader  string  `json:"gapToLeader,omitempty"`
	DurationSec  *float64 `json:"durationSec,omitempty"`
}

type F1Event struct {
	ID           string `json:"id"`
	Date         string `json:"date"`
	Category     string `json:"category"`
	Flag         string `json:"flag,omitempty"`
	Scope        string `json:"scope,omitempty"`
	LapNumber    *int   `json:"lapNumber,omitempty"`
	DriverNumber *int   `json:"driverNumber,omitempty"`
	DriverName   string `json:"driverName,omitempty"`
	Message      string `json:"message"`
	Significant  bool   `json:"significant"`
}

type F1RaceDetail struct {
	Race    F1Race           `json:"race"`
	Results []F1DriverResult `json:"results"`
	Events  []F1Event        `json:"events"`
}
