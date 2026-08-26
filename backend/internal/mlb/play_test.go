package mlb

import (
	"encoding/json"
	"testing"
)

func TestNormalizeLiveFeedIncludesPitcherOnPlay(t *testing.T) {
	fixture := []byte(`{
		"gamePk": 123,
		"gameData": {
			"game": {"season": "2026"},
			"datetime": {"dateTime": "2026-08-24T19:00:00Z", "officialDate": "2026-08-24"},
			"status": {"abstractGameState": "Final", "codedGameState": "F", "detailedState": "Final"},
			"teams": {
				"away": {"id": 1, "name": "Away", "abbreviation": "AWY"},
				"home": {"id": 2, "name": "Home", "abbreviation": "HME"}
			}
		},
		"liveData": {
			"linescore": {"teams": {"away": {"runs": 1}, "home": {"runs": 0}}},
			"plays": {"allPlays": [{
				"result": {"event": "Single", "description": "A single.", "awayScore": 1, "homeScore": 0},
				"about": {"atBatIndex": 7, "halfInning": "top", "inning": 3, "isScoringPlay": true},
				"matchup": {"pitcher": {"id": 99, "fullName": "Casey Pitcher"}}
			}]},
			"boxscore": {"teams": {"away": {}, "home": {}}}
		}
	}`)

	var raw liveFeed
	if err := json.Unmarshal(fixture, &raw); err != nil {
		t.Fatal(err)
	}
	detail := normalizeLiveFeed(raw)
	if len(detail.Plays) != 1 {
		t.Fatalf("expected one play, got %d", len(detail.Plays))
	}
	if detail.Plays[0].PitcherID != 99 || detail.Plays[0].PitcherName != "Casey Pitcher" {
		t.Fatalf("pitcher was not normalized: %+v", detail.Plays[0])
	}
}
