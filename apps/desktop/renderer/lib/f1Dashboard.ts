import type {
  F1DriverStanding,
  F1DriverResult,
  F1Race,
  F1TeamStanding,
} from "@rss-reader/shared";

export interface F1ConstructorWithDrivers extends F1TeamStanding {
  drivers: F1DriverStanding[];
}

function raceTime(race: F1Race): number {
  const value = Date.parse(race.dateStart);
  return Number.isNaN(value) ? 0 : value;
}

export function chronologicalRaces(races: F1Race[]): F1Race[] {
  return [...races].sort((a, b) => raceTime(a) - raceTime(b));
}

export function preferredRaceMeetingKey(
  races: F1Race[],
): number | null {
  const ordered = chronologicalRaces(races);
  if (ordered.length === 0) return null;

  const completed = ordered.filter((race) => race.status === "completed");
  if (completed.length > 0) return completed[completed.length - 1].meetingKey;

  const live = ordered.find((race) => race.status === "in_progress");
  return live?.meetingKey ?? ordered[0].meetingKey;
}

export function classificationResults(results: F1DriverResult[]): F1DriverResult[] {
  return results
    .map((result, index) => ({ result, index }))
    .sort((a, b) => {
      const aOut = a.result.dnf || a.result.dns || a.result.dsq;
      const bOut = b.result.dnf || b.result.dns || b.result.dsq;
      if (aOut !== bOut) return aOut ? 1 : -1;
      if (aOut && bOut) {
        const aHasPosition = a.result.position > 0;
        const bHasPosition = b.result.position > 0;
        if (aHasPosition !== bHasPosition) return aHasPosition ? -1 : 1;
      }
      return a.result.position - b.result.position || a.index - b.index;
    })
    .map(({ result }) => result);
}

function normalizedTeamName(name: string | undefined): string {
  return (name ?? "").trim().toLocaleLowerCase();
}

export function constructorsWithDrivers(
  constructors: F1TeamStanding[],
  drivers: F1DriverStanding[],
): F1ConstructorWithDrivers[] {
  return constructors.map((constructor) => ({
    ...constructor,
    drivers: drivers
      .filter(
        (driver) =>
          normalizedTeamName(driver.teamName) === normalizedTeamName(constructor.teamName),
      )
      .sort((a, b) => b.points - a.points || a.position - b.position),
  }));
}

export function driverContributionPercent(driverPoints: number, teamPoints: number): number {
  if (teamPoints <= 0) return 0;
  return Math.max(0, Math.min(100, (driverPoints / teamPoints) * 100));
}
