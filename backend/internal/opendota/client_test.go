package opendota_test

import (
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/opendota"
)

func TestFindInProMatchesTIGame(t *testing.T) {
	list := []opendota.ProMatch{
		{MatchID: 8955796155, StartTime: 1787232121, Duration: 2501, RadiantName: "Team Falcons", DireName: "Nigma Galaxy"},
		{MatchID: 1, StartTime: 1787232121, Duration: 2501, RadiantName: "Other", DireName: "Teams"},
	}
	id, conf, ok := opendota.FindInProMatches(list, 1787232268, 2501, "Nigma Galaxy", "NGX", "Team Falcons", "FLC")
	if !ok || id != 8955796155 || conf != "inferred" {
		t.Fatalf("got id=%d conf=%q ok=%v", id, conf, ok)
	}
}

func TestFindInTeamMatchesSpiritXG(t *testing.T) {
	list := []opendota.TeamMatch{
		{MatchID: 8943143428, StartTime: 1786605530, Duration: 3054, OpposingTeamName: "Xtreme Gaming"},
		{MatchID: 99, StartTime: 1786605530, Duration: 3054, OpposingTeamName: "Other"},
	}
	id, ok := opendota.FindInTeamMatches(list, 1786605611, 3054, "Xtreme Gaming", "XG")
	if !ok || id != 8943143428 {
		t.Fatalf("got id=%d ok=%v", id, ok)
	}
}
