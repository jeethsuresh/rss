import { describe, expect, test } from "bun:test";
import { mlbTeamHref, mlbTeamRouteFromHash, selectedSportsYear } from "./sportsDeepLinks";

describe("MLB team deep links", () => {
  test("builds a team route without a query string", () => {
    const href = mlbTeamHref(141);
    expect(href).toBe("#/sports/mlb/teams/141");
    expect(mlbTeamRouteFromHash(href)).toEqual({ teamId: 141, season: null });
  });

  test("still parses an older season query without putting it back in the href", () => {
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/141?season=2026")).toEqual({
      teamId: 141,
      season: 2026,
    });
  });

  test("allows a team route without a season", () => {
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/147")).toEqual({
      teamId: 147,
      season: null,
    });
  });

  test("opens on the calendar year unless the user picked another listed year", () => {
    const now = new Date(2026, 9, 1);
    expect(selectedSportsYear(null, [1876, 2025, 2026], now)).toBe(2026);
    expect(selectedSportsYear(2026, [1876, 2025], now)).toBe(2026);
    expect(selectedSportsYear(2025, [1876, 2025, 2026], now)).toBe(2025);
  });

  test("rejects malformed team routes", () => {
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/0")).toBeNull();
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/not-a-team")).toBeNull();
    expect(mlbTeamRouteFromHash("#/sports/f1/teams/1")).toBeNull();
  });
});
