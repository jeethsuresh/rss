import type { MlbGame, MlbGameStatus, MlbPlay } from "@rss-reader/shared";

export type MlbLeague = "AL" | "NL";
export type MlbGameResult = "win" | "loss";

export interface PitcherChange {
  playId: string;
  inning: number;
  half: string;
  pitcherId?: number;
  pitcherName: string;
}

export type MlbFieldingSide = "away" | "home";

export interface PitcherStint {
  key: string;
  fieldingSide: MlbFieldingSide;
  pitcherId?: number;
  pitcherName: string;
  start: number;
  end: number;
  startInning: number;
  endInning: number;
  startingPitcher: boolean;
}

export function localDateKey(date = new Date()): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function shiftLocalDateKey(dateKey: string, days: number): string {
  const date = new Date(`${dateKey}T12:00:00`);
  if (Number.isNaN(date.getTime())) return dateKey;
  date.setDate(date.getDate() + days);
  return localDateKey(date);
}

export function gameDateKey(game: MlbGame): string {
  return game.officialDate || game.gameDate.slice(0, 10);
}

export function chronologicalGames(games: MlbGame[]): MlbGame[] {
  return [...games].sort((a, b) => {
    const aTime = Date.parse(a.gameDate);
    const bTime = Date.parse(b.gameDate);
    if (Number.isNaN(aTime) || Number.isNaN(bTime)) {
      return gameDateKey(a).localeCompare(gameDateKey(b));
    }
    return aTime - bTime;
  });
}

export function teamGameResult(game: MlbGame, teamId: number): MlbGameResult | null {
  if (
    game.status !== "final"
    || game.awayScore == null
    || game.homeScore == null
    || game.awayScore === game.homeScore
  ) {
    return null;
  }

  if (game.awayTeam.id === teamId) {
    return game.awayScore > game.homeScore ? "win" : "loss";
  }
  if (game.homeTeam.id === teamId) {
    return game.homeScore > game.awayScore ? "win" : "loss";
  }
  return null;
}

function sameDayPriority(status: MlbGameStatus): number {
  switch (status) {
    case "live":
      return 0;
    case "pre_game":
    case "scheduled":
      return 1;
    case "final":
      return 2;
    case "postponed":
    case "cancelled":
      return 3;
    case "unknown":
      return 4;
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}

export function preferredGameId(games: MlbGame[], now = new Date()): number | null {
  const ordered = chronologicalGames(games);
  if (ordered.length === 0) return null;

  const today = localDateKey(now);
  const todayGames = ordered
    .filter((game) => gameDateKey(game) === today)
    .sort((a, b) => sameDayPriority(a.status) - sameDayPriority(b.status));
  if (todayGames.length > 0) return todayGames[0].id;

  const next = ordered.find((game) => gameDateKey(game) > today);
  return next?.id ?? ordered[ordered.length - 1].id;
}

export function pitcherChanges(plays: MlbPlay[]): PitcherChange[] {
  const lastPitcherByFieldingSide = new Map<"away" | "home", string>();
  const changes: PitcherChange[] = [];

  for (const play of plays) {
    if (!play.pitcherId && !play.pitcherName) continue;
    const fieldingSide = play.half === "top" ? "home" : "away";
    const pitcherKey = play.pitcherId ? String(play.pitcherId) : play.pitcherName || "";
    const previous = lastPitcherByFieldingSide.get(fieldingSide);
    if (previous && previous !== pitcherKey) {
      changes.push({
        playId: play.id,
        inning: play.inning,
        half: play.half,
        pitcherId: play.pitcherId,
        pitcherName: play.pitcherName || "New pitcher",
      });
    }
    lastPitcherByFieldingSide.set(fieldingSide, pitcherKey);
  }

  return changes;
}

export function pitcherStints(plays: MlbPlay[]): PitcherStint[] {
  const halfCounts = new Map<string, number>();
  for (const play of plays) {
    if (!play.pitcherId && !play.pitcherName) continue;
    const key = `${play.inning}-${play.half}`;
    halfCounts.set(key, (halfCounts.get(key) ?? 0) + 1);
  }

  const seenInHalf = new Map<string, number>();
  const playsBySide = new Map<MlbFieldingSide, Array<{ play: MlbPlay; start: number; end: number }>>([
    ["away", []],
    ["home", []],
  ]);

  for (const play of plays) {
    if (!play.pitcherId && !play.pitcherName) continue;
    const halfKey = `${play.inning}-${play.half}`;
    const count = halfCounts.get(halfKey) ?? 1;
    const seen = seenInHalf.get(halfKey) ?? 0;
    seenInHalf.set(halfKey, seen + 1);
    const fieldingSide: MlbFieldingSide = play.half === "top" ? "home" : "away";
    playsBySide.get(fieldingSide)?.push({
      play,
      start: play.inning - 1 + seen / count,
      end: play.inning - 1 + (seen + 1) / count,
    });
  }

  const stints: PitcherStint[] = [];
  for (const fieldingSide of ["away", "home"] as const) {
    const sidePlays = playsBySide.get(fieldingSide) ?? [];
    let active: PitcherStint | null = null;
    let stintIndex = 0;

    for (const item of sidePlays) {
      const pitcherKey = item.play.pitcherId
        ? String(item.play.pitcherId)
        : item.play.pitcherName || "unknown";
      const activeKey: string = active?.pitcherId
        ? String(active.pitcherId)
        : active?.pitcherName || "";

      if (!active || activeKey !== pitcherKey) {
        if (active) {
          active.end = item.start;
          active.endInning = Math.max(active.startInning, Math.ceil(item.start));
          stints.push(active);
        }
        active = {
          key: `${fieldingSide}-${pitcherKey}-${stintIndex}`,
          fieldingSide,
          pitcherId: item.play.pitcherId,
          pitcherName: item.play.pitcherName || "Unknown pitcher",
          start: stintIndex === 0 ? 0 : item.start,
          end: item.end,
          startInning: stintIndex === 0 ? 1 : item.play.inning,
          endInning: item.play.inning,
          startingPitcher: stintIndex === 0,
        };
        stintIndex += 1;
      } else {
        active.end = item.end;
        active.endInning = item.play.inning;
      }
    }
    if (active) stints.push(active);
  }

  return stints;
}
