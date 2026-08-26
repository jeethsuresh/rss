import { describe, expect, test } from "bun:test";
import { mlbTeamHref, mlbTeamRouteFromHash } from "./sportsDeepLinks";

describe("MLB team deep links", () => {
  test("builds and parses a team route with its season", () => {
    const href = mlbTeamHref(141, 2026);
    expect(href).toBe("#/sports/mlb/teams/141?season=2026");
    expect(mlbTeamRouteFromHash(href)).toEqual({ teamId: 141, season: 2026 });
  });

  test("allows a team route without a season", () => {
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/147")).toEqual({
      teamId: 147,
      season: null,
    });
  });

  test("rejects malformed team routes", () => {
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/0")).toBeNull();
    expect(mlbTeamRouteFromHash("#/sports/mlb/teams/not-a-team")).toBeNull();
    expect(mlbTeamRouteFromHash("#/sports/f1/teams/1")).toBeNull();
  });
});
