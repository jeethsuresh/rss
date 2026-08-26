export type MlbTeamRoute = {
  teamId: number;
  season: number | null;
};

export function mlbTeamHref(teamId: number, season?: number | null): string {
  const seasonQuery = season == null ? "" : `?season=${encodeURIComponent(String(season))}`;
  return `#/sports/mlb/teams/${teamId}${seasonQuery}`;
}

export function mlbTeamRouteFromHash(hash: string): MlbTeamRoute | null {
  const match = /^#\/sports\/mlb\/teams\/(\d+)(?:\?(.+))?$/.exec(hash);
  if (!match) return null;

  const teamId = Number(match[1]);
  if (!Number.isSafeInteger(teamId) || teamId <= 0) return null;

  const rawSeason = new URLSearchParams(match[2] ?? "").get("season");
  const parsedSeason = rawSeason == null ? Number.NaN : Number(rawSeason);
  return {
    teamId,
    season: Number.isSafeInteger(parsedSeason) && parsedSeason > 0 ? parsedSeason : null,
  };
}
