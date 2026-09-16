import { describe, expect, test } from "bun:test";
import type { MlbGame, MlbPlay, MlbStandingRow, MlbStandings, MlbTeam } from "@rss-reader/shared";
import {
  chronologicalGames,
  pitcherChanges,
  pitcherStints,
  preferredGameId,
  shiftLocalDateKey,
  teamGameResult,
  teamPlayoffOutlook,
} from "./baseballDashboard";

const away: MlbTeam = { id: 1, name: "Away", abbreviation: "AWY" };
const home: MlbTeam = { id: 2, name: "Home", abbreviation: "HME" };

function game(id: number, gameDate: string, status: MlbGame["status"]): MlbGame {
  return { id, season: 2026, gameDate, status, awayTeam: away, homeTeam: home };
}

describe("baseball dashboard schedule", () => {
  test("moves between local schedule dates", () => {
    expect(shiftLocalDateKey("2026-03-01", -1)).toBe("2026-02-28");
    expect(shiftLocalDateKey("2026-12-31", 1)).toBe("2027-01-01");
  });

  test("sorts the rail chronologically", () => {
    const games = [
      game(3, "2026-08-25T23:00:00Z", "scheduled"),
      game(1, "2026-08-23T23:00:00Z", "final"),
      game(2, "2026-08-24T23:00:00Z", "final"),
    ];
    expect(chronologicalGames(games).map((item) => item.id)).toEqual([1, 2, 3]);
  });

  test("focuses today's game even after it is final", () => {
    const games = [
      game(1, "2026-08-23T23:00:00Z", "final"),
      game(2, "2026-08-24T23:00:00Z", "final"),
      game(3, "2026-08-25T23:00:00Z", "scheduled"),
    ];
    expect(preferredGameId(games, new Date(2026, 7, 24, 12))).toBe(2);
  });

  test("prefers the live game during a doubleheader", () => {
    const games = [
      game(1, "2026-08-24T17:00:00Z", "final"),
      game(2, "2026-08-24T23:00:00Z", "live"),
    ];
    expect(preferredGameId(games, new Date(2026, 7, 24, 12))).toBe(2);
  });

  test("reports a completed result from the selected team's perspective", () => {
    const completed = {
      ...game(1, "2026-08-24T23:00:00Z", "final"),
      awayScore: 3,
      homeScore: 8,
    };
    expect(teamGameResult(completed, away.id)).toBe("loss");
    expect(teamGameResult(completed, home.id)).toBe("win");
  });

  test("does not show a result before a game is final", () => {
    const live = {
      ...game(1, "2026-08-24T23:00:00Z", "live"),
      awayScore: 3,
      homeScore: 8,
    };
    expect(teamGameResult(live, away.id)).toBeNull();
  });
});

function standing(team: MlbTeam, rank: number, wins: number, losses: number, divisionLeader = false): MlbStandingRow {
  return {
    rank,
    team,
    wins,
    losses,
    winningPercentage: (wins / (wins + losses)).toFixed(3),
    gamesBack: rank === 1 ? "-" : "1.0",
    wildCardGamesBack: rank === 1 ? "-" : "1.0",
    runDifferential: 0,
    divisionLeader,
  };
}

const eastLeader: MlbTeam = { id: 10, name: "East Leader", abbreviation: "EL" };
const selected: MlbTeam = { id: 11, name: "Selected", abbreviation: "SEL" };
const centralLeader: MlbTeam = { id: 20, name: "Central Leader", abbreviation: "CL" };
const westLeader: MlbTeam = { id: 30, name: "West Leader", abbreviation: "WL" };
const wildOne: MlbTeam = { id: 12, name: "Wild One", abbreviation: "W1" };
const wildThree: MlbTeam = { id: 13, name: "Wild Three", abbreviation: "W3" };
const wildFour: MlbTeam = { id: 14, name: "Wild Four", abbreviation: "W4" };

function standingsFor(selectedRow: MlbStandingRow, wildCardRows: MlbStandingRow[]): MlbStandings {
  return {
    season: 2026,
    sections: [
      {
        id: "div-1", league: "AL", name: "East", kind: "division",
        teams: [standing(eastLeader, 1, 80, 50, true), selectedRow],
      },
      {
        id: "div-2", league: "AL", name: "Central", kind: "division",
        teams: [standing(centralLeader, 1, 78, 52, true), standing(wildOne, 2, 77, 53)],
      },
      {
        id: "div-3", league: "AL", name: "West", kind: "division",
        teams: [standing(westLeader, 1, 79, 51, true), standing(wildThree, 2, 74, 56), standing(wildFour, 3, 72, 58)],
      },
      { id: "wc-1", league: "AL", name: "Wild Card", kind: "wildcard", teams: wildCardRows },
    ],
  };
}

describe("baseball playoff outlook", () => {
  test("reports a division leader's margin over second place", () => {
    const data = standingsFor(
      standing(selected, 2, 75, 55),
      [standing(wildOne, 1, 77, 53), standing(selected, 2, 75, 55), standing(wildThree, 3, 74, 56), standing(wildFour, 4, 72, 58)],
    );
    data.sections[0].teams = [standing(selected, 1, 80, 50, true), standing(eastLeader, 2, 75, 55)];

    expect(teamPlayoffOutlook(data, selected.id)).toEqual({
      kind: "division",
      title: "Projected playoff team",
      detail: "AL East leader · 5 games ahead",
    });
  });

  test("reports wild-card position and cushion over the first team out", () => {
    const data = standingsFor(
      standing(selected, 2, 75, 55),
      [standing(wildOne, 1, 77, 53), standing(selected, 2, 75, 55), standing(wildThree, 3, 74, 56), standing(wildFour, 4, 72, 58)],
    );

    expect(teamPlayoffOutlook(data, selected.id)).toEqual({
      kind: "wildcard",
      title: "Projected wild card",
      detail: "2nd wild card · 3 games ahead of first team out",
    });
  });

  test("reports the gap to the final wild-card spot when outside the field", () => {
    const outside = standing(selected, 2, 70, 60);
    const data = standingsFor(
      outside,
      [standing(wildOne, 1, 77, 53), standing(wildThree, 2, 74, 56), standing(wildFour, 3, 72, 58), { ...outside, rank: 4 }],
    );

    expect(teamPlayoffOutlook(data, selected.id)).toEqual({
      kind: "outside",
      title: "Outside playoff field",
      detail: "2 games behind the final wild-card spot",
    });
  });

  test("keeps half-game cutoff margins", () => {
    const halfGameBack = standing(selected, 2, 71, 57);
    const data = standingsFor(
      halfGameBack,
      [standing(wildOne, 1, 77, 53), standing(wildThree, 2, 74, 56), standing(wildFour, 3, 72, 57), { ...halfGameBack, rank: 4 }],
    );

    expect(teamPlayoffOutlook(data, selected.id)?.detail).toBe("0.5 games behind the final wild-card spot");
  });
});

describe("pitching changes", () => {
  test("detects changes independently for each fielding team", () => {
    const plays: MlbPlay[] = [
      { id: "1", inning: 1, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 10, pitcherName: "A" },
      { id: "2", inning: 1, half: "bottom", event: "Out", description: "", isScoringPlay: false, pitcherId: 20, pitcherName: "B" },
      { id: "3", inning: 2, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 11, pitcherName: "C" },
      { id: "4", inning: 2, half: "bottom", event: "Out", description: "", isScoringPlay: false, pitcherId: 20, pitcherName: "B" },
    ];
    expect(pitcherChanges(plays)).toEqual([
      { playId: "3", inning: 2, half: "top", pitcherId: 11, pitcherName: "C" },
    ]);
  });

  test("builds continuous stints including both starting pitchers", () => {
    const plays: MlbPlay[] = [
      { id: "1", inning: 1, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 10, pitcherName: "Home Starter" },
      { id: "2", inning: 1, half: "bottom", event: "Out", description: "", isScoringPlay: false, pitcherId: 20, pitcherName: "Away Starter" },
      { id: "3", inning: 2, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 10, pitcherName: "Home Starter" },
      { id: "4", inning: 2, half: "bottom", event: "Out", description: "", isScoringPlay: false, pitcherId: 20, pitcherName: "Away Starter" },
      { id: "5", inning: 3, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 11, pitcherName: "Home Reliever" },
    ];
    expect(pitcherStints(plays)).toEqual([
      {
        key: "away-20-0",
        fieldingSide: "away",
        pitcherId: 20,
        pitcherName: "Away Starter",
        start: 0,
        end: 2,
        startInning: 1,
        endInning: 2,
        startingPitcher: true,
      },
      {
        key: "home-10-0",
        fieldingSide: "home",
        pitcherId: 10,
        pitcherName: "Home Starter",
        start: 0,
        end: 2,
        startInning: 1,
        endInning: 2,
        startingPitcher: true,
      },
      {
        key: "home-11-1",
        fieldingSide: "home",
        pitcherId: 11,
        pitcherName: "Home Reliever",
        start: 2,
        end: 3,
        startInning: 3,
        endInning: 3,
        startingPitcher: false,
      },
    ]);
  });

  test("splits pitcher lines within an inning at the plate-appearance boundary", () => {
    const plays: MlbPlay[] = [
      { id: "1", inning: 1, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 10, pitcherName: "Starter" },
      { id: "2", inning: 1, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 11, pitcherName: "Reliever" },
      { id: "3", inning: 1, half: "top", event: "Out", description: "", isScoringPlay: false, pitcherId: 11, pitcherName: "Reliever" },
    ];
    const stints = pitcherStints(plays);
    expect(stints[0].start).toBe(0);
    expect(stints[0].end).toBeCloseTo(1 / 3);
    expect(stints[1].start).toBeCloseTo(1 / 3);
    expect(stints[1].end).toBe(1);
  });
});
