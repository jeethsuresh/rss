package opendota_test

import (
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/opendota"
)

func TestEventsForYearPrefersPremium(t *testing.T) {
	leagues := []opendota.League{
		{LeagueID: 1, Name: "Amateur Cup 2026", Tier: "amateur"},
		{LeagueID: 2, Name: "The International 2026", Tier: "premium"},
		{LeagueID: 3, Name: "Pro Circuit 2026", Tier: "professional"},
		{LeagueID: 4, Name: "Old League 2024", Tier: "premium"},
	}
	ev := opendota.EventsForYear(leagues, 2026)
	if len(ev) != 3 {
		t.Fatalf("want 3 events, got %d", len(ev))
	}
	if ev[0].ID != "2" || ev[0].Tier != domain.DotaTierPremier {
		t.Fatalf("expected TI premium first, got %+v", ev[0])
	}
	if ev[1].ID != "3" {
		t.Fatalf("expected pro second, got %+v", ev[1])
	}
}

func TestLeagueYear(t *testing.T) {
	if y := opendota.LeagueYear("The International 2026"); y != 2026 {
		t.Fatalf("got %d", y)
	}
	if y := opendota.LeagueYear("no year here"); y != 0 {
		t.Fatalf("got %d", y)
	}
}

func TestSeriesFromLeagueMatchesGroups(t *testing.T) {
	rows := []opendota.LeagueMatchRow{
		{
			MatchID: 100, SeriesID: 50, StartTime: 1000, Duration: 2000,
			RadiantTeamID: 1, DireTeamID: 2, RadiantWin: true, LeagueID: 9,
		},
		{
			MatchID: 101, SeriesID: 50, StartTime: 2000, Duration: 2100,
			RadiantTeamID: 2, DireTeamID: 1, RadiantWin: true, LeagueID: 9,
		},
		{
			MatchID: 200, SeriesID: 0, StartTime: 3000, Duration: 1800,
			RadiantTeamID: 3, DireTeamID: 4, RadiantWin: false, LeagueID: 9,
		},
	}
	teams := []opendota.LeagueTeam{
		{TeamID: 1, Name: "Spirit", Tag: "TS"},
		{TeamID: 2, Name: "Falcons", Tag: "FLC"},
		{TeamID: 3, Name: "XG", Tag: "XG"},
		{TeamID: 4, Name: "Tidebound", Tag: "TT"},
	}
	ms := opendota.SeriesFromLeagueMatches(rows, teams, 2026)
	if len(ms) != 2 {
		t.Fatalf("want 2 series, got %d", len(ms))
	}
	var series50 *domain.DotaMatch
	for i := range ms {
		if ms[i].ID == 50 {
			series50 = &ms[i]
		}
	}
	if series50 == nil {
		t.Fatal("missing series 50")
	}
	if series50.ScoreA == nil || series50.ScoreB == nil || *series50.ScoreA != 1 || *series50.ScoreB != 1 {
		t.Fatalf("want 1-1, got %v-%v", series50.ScoreA, series50.ScoreB)
	}
	games := opendota.GamesForSeries(rows, 50, teams)
	if len(games) != 2 {
		t.Fatalf("want 2 games, got %d", len(games))
	}
	if games[0].WinnerTeamName != "Spirit" {
		t.Fatalf("game1 winner %q", games[0].WinnerTeamName)
	}
}
