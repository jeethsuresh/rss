import { describe, expect, test } from "bun:test";
import type { MlbGame, MlbPlay, MlbTeam } from "@rss-reader/shared";
import {
  chronologicalGames,
  pitcherChanges,
  pitcherStints,
  preferredGameId,
  shiftLocalDateKey,
  teamGameResult,
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
