export type MlbTeamRoute = {
  teamId: number;
  season: number | null;
};

/** In-page team route. Season stays in the year control, not the hash.
 *  A `?query` inside the fragment makes Firefox resolve the link as `file:///`. */
export function mlbTeamHref(teamId: number): string {
  return `#/sports/mlb/teams/${teamId}`;
}

export function currentCalendarYear(now = new Date()): number {
  return now.getFullYear();
}

/** Use the calendar year unless the user has chosen a different listed year. */
export function selectedSportsYear(
  previous: number | null,
  available: readonly number[],
  now = new Date(),
): number {
  const year = currentCalendarYear(now);
  if (previous != null && previous !== year && available.includes(previous)) return previous;
  return year;
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
