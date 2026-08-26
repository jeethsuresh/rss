package mlb

import "testing"

func TestNormalizeScheduleGameIncludesInningScores(t *testing.T) {
	awayOne, homeZero, awayTwo := 1, 0, 2
	raw := scheduleGame{GamePk: 123, Season: "2026"}
	raw.Linescore = &struct {
		CurrentInning int    `json:"currentInning"`
		IsTopInning   bool   `json:"isTopInning"`
		InningHalf    string `json:"inningHalf"`
		Innings       []struct {
			Num  int `json:"num"`
			Away struct {
				Runs *int `json:"runs"`
			} `json:"away"`
			Home struct {
				Runs *int `json:"runs"`
			} `json:"home"`
		} `json:"innings"`
	}{CurrentInning: 2, IsTopInning: true, InningHalf: "Top"}
	raw.Linescore.Innings = make([]struct {
		Num  int `json:"num"`
		Away struct {
			Runs *int `json:"runs"`
		} `json:"away"`
		Home struct {
			Runs *int `json:"runs"`
		} `json:"home"`
	}, 2)
	raw.Linescore.Innings[0].Num = 1
	raw.Linescore.Innings[0].Away.Runs = &awayOne
	raw.Linescore.Innings[0].Home.Runs = &homeZero
	raw.Linescore.Innings[1].Num = 2
	raw.Linescore.Innings[1].Away.Runs = &awayTwo

	game := normalizeScheduleGame(raw)
	if len(game.InningScores) != 2 {
		t.Fatalf("expected 2 inning scores, got %d", len(game.InningScores))
	}
	if game.InningScores[0].AwayRuns == nil || *game.InningScores[0].AwayRuns != 1 {
		t.Fatalf("expected away run in first inning")
	}
	if game.InningScores[1].HomeRuns != nil {
		t.Fatalf("expected unfinished home half to remain unset")
	}
}
