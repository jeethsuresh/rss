import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  F1Race,
  F1RaceDetail,
  F1RaceStatus,
  F1Season,
  F1Session,
  F1SessionKind,
  F1Standings,
  MlbGame,
  MlbGameDetail,
  MlbGameStatus,
  MlbSeason,
  MlbStandingSection,
  MlbStandings,
  MlbTeam,
  MlbTeamBox,
  ReaderBackend,
  SportsCacheUpdatedEvent,
  SportsRefreshEvent,
} from "@rss-reader/shared";
import { SportsLoadingPane, SportsSpinner } from "../components/SportsSpinner";
import { SPORTS_REGISTRY, type SportId } from "../lib/sportsRegistry";

type Props = {
  backend: ReaderBackend;
  onOpenSettingsSports?: () => void;
};

type GameBucket = "completed" | "in_progress" | "scheduled";

type MlbSelection =
  | { type: "all"; bucket: GameBucket }
  | { type: "team"; id: number; bucket: GameBucket }
  | { type: "standings"; sectionId: string };

type F1Mode = "races" | "wdc" | "wcc";
type MlbDetailTab = "plays" | "stats";
type F1DetailTab = "classification" | "events";

const F1_KIND_PILLS: { id: F1SessionKind; label: string }[] = [
  { id: "practice", label: "Practice" },
  { id: "sprint_quali", label: "Sprint Quali" },
  { id: "quali", label: "Quali" },
  { id: "race", label: "Race" },
];

function sessionMatchesKind(session: F1Session, kind: F1SessionKind): boolean {
  if (kind === "race") return session.kind === "race" || session.kind === "sprint";
  return session.kind === kind;
}

function sessionsForKind(sessions: F1Session[], kind: F1SessionKind): F1Session[] {
  return sessions.filter((s) => sessionMatchesKind(s, kind));
}

function defaultKind(sessions: F1Session[]): F1SessionKind {
  if (sessions.some((s) => s.kind === "race")) return "race";
  if (sessions.some((s) => s.kind === "sprint")) return "race";
  if (sessions.some((s) => s.kind === "quali")) return "quali";
  if (sessions.some((s) => s.kind === "sprint_quali")) return "sprint_quali";
  if (sessions.some((s) => s.kind === "practice")) return "practice";
  return "race";
}

const BUCKETS: { id: GameBucket; label: string }[] = [
  { id: "completed", label: "Completed" },
  { id: "in_progress", label: "In Progress" },
  { id: "scheduled", label: "Scheduled" },
];

function gameBucket(status: MlbGameStatus): GameBucket {
  switch (status) {
    case "final":
    case "postponed":
    case "cancelled":
      return "completed";
    case "live":
      return "in_progress";
    case "scheduled":
    case "pre_game":
    case "unknown":
      return "scheduled";
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}

function f1Bucket(status: F1RaceStatus): GameBucket {
  switch (status) {
    case "completed":
    case "cancelled":
      return "completed";
    case "in_progress":
      return "in_progress";
    case "scheduled":
      return "scheduled";
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}

function statusLabel(status: MlbGameStatus, detail?: string): string {
  switch (status) {
    case "live":
      return detail || "Live";
    case "final":
      return "Final";
    case "scheduled":
      return "Scheduled";
    case "pre_game":
      return detail || "Pre-game";
    case "postponed":
      return "Postponed";
    case "cancelled":
      return "Cancelled";
    case "unknown":
      return detail || "Unknown";
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}

function f1StatusLabel(status: F1RaceStatus): string {
  switch (status) {
    case "in_progress":
      return "In progress";
    case "completed":
      return "Finished";
    case "scheduled":
      return "Scheduled";
    case "cancelled":
      return "Cancelled";
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}

function scoreText(g: MlbGame): string {
  if (g.awayScore == null || g.homeScore == null) return "—";
  return `${g.awayScore}–${g.homeScore}`;
}

function matchup(g: MlbGame): string {
  const away = g.awayTeam.abbreviation || g.awayTeam.shortName || g.awayTeam.name;
  const home = g.homeTeam.abbreviation || g.homeTeam.shortName || g.homeTeam.name;
  return `${away} @ ${home}`;
}

function resultStatus(r: F1RaceDetail["results"][number]): string {
  if (r.dsq) return "DSQ";
  if (r.dns) return "DNS";
  if (r.dnf) return "DNF";
  return r.gapToLeader || "";
}

function TeamBoxSection({ box }: { box: MlbTeamBox }) {
  const label = box.team.abbreviation || box.team.shortName || box.team.name;
  return (
    <div className="sports-team-box">
      <h2 className="sports-plays-heading">{label}</h2>
      <h3 className="sports-box-subheading">Hitting</h3>
      {box.batters.length === 0 ? (
        <p className="muted">No hitting lines yet.</p>
      ) : (
        <div className="sports-linescore-wrap">
          <table className="sports-linescore sports-standings-table sports-box-table">
            <thead>
              <tr>
                <th>Player</th>
                <th>Pos</th>
                <th>AB</th>
                <th>R</th>
                <th>H</th>
                <th>RBI</th>
                <th>BB</th>
                <th>SO</th>
                <th>HR</th>
              </tr>
            </thead>
            <tbody>
              {box.batters.map((b) => (
                <tr key={b.playerId}>
                  <td className="sports-box-player">{b.name}</td>
                  <td>{b.position || "—"}</td>
                  <td>{b.atBats}</td>
                  <td>{b.runs}</td>
                  <td>{b.hits}</td>
                  <td>{b.rbi}</td>
                  <td>{b.walks}</td>
                  <td>{b.strikeOuts}</td>
                  <td>{b.homeRuns}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <h3 className="sports-box-subheading">Pitching</h3>
      {box.pitchers.length === 0 ? (
        <p className="muted">No pitching lines yet.</p>
      ) : (
        <div className="sports-linescore-wrap">
          <table className="sports-linescore sports-standings-table sports-box-table">
            <thead>
              <tr>
                <th>Pitcher</th>
                <th>IP</th>
                <th>H</th>
                <th>R</th>
                <th>ER</th>
                <th>BB</th>
                <th>K</th>
                <th>HR</th>
                <th>P</th>
              </tr>
            </thead>
            <tbody>
              {box.pitchers.map((p) => (
                <tr key={p.playerId}>
                  <td className="sports-box-player">
                    {p.name}
                    {p.note ? (
                      <span className="sports-scoring-badge" style={{ marginLeft: 6 }}>
                        {p.note}
                      </span>
                    ) : null}
                  </td>
                  <td>{p.inningsPitched || "—"}</td>
                  <td>{p.hits}</td>
                  <td>{p.runs}</td>
                  <td>{p.earnedRuns}</td>
                  <td>{p.walks}</td>
                  <td>{p.strikeOuts}</td>
                  <td>{p.homeRuns}</td>
                  <td>{p.pitchesThrown || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export function SportsView({ backend, onOpenSettingsSports }: Props) {
  const [activeSport, setActiveSport] = useState<SportId>("mlb");

  // --- MLB ---
  const [teams, setTeams] = useState<MlbTeam[]>([]);
  const [followed, setFollowed] = useState<number[]>([]);
  const [seasons, setSeasons] = useState<MlbSeason[]>([]);
  const [season, setSeason] = useState<number | null>(null);
  const [selection, setSelection] = useState<MlbSelection>({ type: "all", bucket: "scheduled" });
  const [games, setGames] = useState<MlbGame[]>([]);
  const [activePk, setActivePk] = useState<number | null>(null);
  const [detail, setDetail] = useState<MlbGameDetail | null>(null);
  const [scoringOnly, setScoringOnly] = useState(false);
  const [mlbDetailTab, setMlbDetailTab] = useState<MlbDetailTab>("plays");
  const [mlbStandings, setMlbStandings] = useState<MlbStandings | null>(null);

  // --- F1 ---
  const [f1Years, setF1Years] = useState<F1Season[]>([]);
  const [f1Year, setF1Year] = useState<number | null>(null);
  const [f1Mode, setF1Mode] = useState<F1Mode>("races");
  const [f1BucketSel, setF1BucketSel] = useState<GameBucket>("completed");
  const [f1Races, setF1Races] = useState<F1Race[]>([]);
  const [activeMeetingKey, setActiveMeetingKey] = useState<number | null>(null);
  const [activeSessionKey, setActiveSessionKey] = useState<number | null>(null);
  const [f1SessionKind, setF1SessionKind] = useState<F1SessionKind>("race");
  const [f1Detail, setF1Detail] = useState<F1RaceDetail | null>(null);
  const [f1DetailTab, setF1DetailTab] = useState<F1DetailTab>("classification");
  const [significantOnly, setSignificantOnly] = useState(true);
  const [f1Standings, setF1Standings] = useState<F1Standings | null>(null);

  const [refreshingKeys, setRefreshingKeys] = useState<ReadonlySet<string>>(() => new Set());
  const syncRefreshKeys = useRef(new Set<string>());
  const [error, setError] = useState<string | null>(null);
  const [expandedSports, setExpandedSports] = useState<ReadonlySet<SportId>>(
    () => new Set<SportId>(["mlb"]),
  );

  const setKeyRefreshing = useCallback((key: string, on: boolean) => {
    setRefreshingKeys((prev) => {
      const has = prev.has(key);
      if (on === has) return prev;
      const next = new Set(prev);
      if (on) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const beginFetch = useCallback(
    (key: string) => {
      syncRefreshKeys.current.add(key);
      setKeyRefreshing(key, true);
    },
    [setKeyRefreshing],
  );

  const endFetchIfSync = useCallback(
    (key: string) => {
      queueMicrotask(() => {
        if (!syncRefreshKeys.current.has(key)) return;
        syncRefreshKeys.current.delete(key);
        setKeyRefreshing(key, false);
      });
    },
    [setKeyRefreshing],
  );

  const isRefreshing = (...keys: string[]) => keys.some((k) => refreshingKeys.has(k));

  const toggleSportExpanded = (id: SportId) => {
    setExpandedSports((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectMlb = (next: MlbSelection) => {
    setActiveSport("mlb");
    setSelection(next);
  };

  const selectF1Races = (bucket: GameBucket) => {
    setActiveSport("f1");
    setF1Mode("races");
    setF1BucketSel(bucket);
  };

  const selectF1Standings = (mode: "wdc" | "wcc") => {
    setActiveSport("f1");
    setF1Mode(mode);
  };

  const followedTeams = useMemo(
    () => teams.filter((t) => followed.includes(t.id)).sort((a, b) => a.name.localeCompare(b.name)),
    [teams, followed],
  );

  const mlbStandingsMode = selection.type === "standings";
  const f1StandingsMode = f1Mode === "wdc" || f1Mode === "wcc";

  const reloadMlbMeta = useCallback(async () => {
    beginFetch("mlb.teams");
    beginFetch("mlb.seasons");
    try {
      const [t, f, s] = await Promise.all([
        backend.sports.teams(),
        backend.sports.followedGet(),
        backend.sports.seasons(),
      ]);
      setTeams(t ?? []);
      setFollowed(f ?? []);
      setSeasons(s ?? []);
      setSeason((prev) => prev ?? s?.[0]?.seasonId ?? new Date().getFullYear());
    } finally {
      endFetchIfSync("mlb.teams");
      endFetchIfSync("mlb.seasons");
    }
  }, [backend, beginFetch, endFetchIfSync]);

  const reloadF1Meta = useCallback(async () => {
    beginFetch("f1.years");
    try {
      const years = await backend.sports.f1Years();
      setF1Years(years ?? []);
      setF1Year((prev) => prev ?? years?.[0]?.year ?? new Date().getFullYear());
    } finally {
      endFetchIfSync("f1.years");
    }
  }, [backend, beginFetch, endFetchIfSync]);

  const scheduleTeamId = selection.type === "team" ? selection.id : undefined;

  const mlbScheduleKey =
    season == null ? "" : `mlb.schedule.${season}.${scheduleTeamId ?? 0}`;
  const mlbStandingsKey = season == null ? "" : `mlb.standings.${season}`;
  const f1RacesKey = f1Year == null ? "" : `f1.races.${f1Year}`;
  const f1StandingsKey = f1Year == null ? "" : `f1.standings.${f1Year}`;

  const reloadMlbSchedule = useCallback(async () => {
    if (season == null || activeSport !== "mlb" || mlbStandingsMode || !mlbScheduleKey) return;
    beginFetch(mlbScheduleKey);
    setError(null);
    setGames([]);
    setActivePk(null);
    setDetail(null);
    try {
      const list = await backend.sports.schedule({ teamId: scheduleTeamId, season });
      setGames(list ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load schedule");
    } finally {
      endFetchIfSync(mlbScheduleKey);
    }
  }, [
    backend,
    season,
    scheduleTeamId,
    activeSport,
    mlbStandingsMode,
    mlbScheduleKey,
    beginFetch,
    endFetchIfSync,
  ]);

  const reloadMlbStandings = useCallback(async () => {
    if (season == null || activeSport !== "mlb" || !mlbStandingsMode || !mlbStandingsKey) return;
    beginFetch(mlbStandingsKey);
    setError(null);
    setMlbStandings(null);
    try {
      const data = await backend.sports.standings({ season });
      setMlbStandings(data);
      setSelection((prev) => {
        if (prev.type !== "standings") return prev;
        if (data.sections.some((s) => s.id === prev.sectionId)) return prev;
        const first = data.sections[0]?.id;
        return first ? { type: "standings", sectionId: first } : prev;
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load standings");
    } finally {
      endFetchIfSync(mlbStandingsKey);
    }
  }, [backend, season, activeSport, mlbStandingsMode, mlbStandingsKey, beginFetch, endFetchIfSync]);

  const reloadF1Races = useCallback(async () => {
    if (f1Year == null || activeSport !== "f1" || f1StandingsMode || !f1RacesKey) return;
    beginFetch(f1RacesKey);
    setError(null);
    setF1Races([]);
    setActiveSessionKey(null);
    setF1Detail(null);
    try {
      const list = await backend.sports.f1Races({ year: f1Year });
      setF1Races(list ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load races");
    } finally {
      endFetchIfSync(f1RacesKey);
    }
  }, [backend, f1Year, activeSport, f1StandingsMode, f1RacesKey, beginFetch, endFetchIfSync]);

  const reloadF1Standings = useCallback(async () => {
    if (f1Year == null || activeSport !== "f1" || !f1StandingsMode || !f1StandingsKey) return;
    beginFetch(f1StandingsKey);
    setError(null);
    setF1Standings(null);
    try {
      const data = await backend.sports.f1Standings({ year: f1Year });
      setF1Standings(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load championship");
    } finally {
      endFetchIfSync(f1StandingsKey);
    }
  }, [backend, f1Year, activeSport, f1StandingsMode, f1StandingsKey, beginFetch, endFetchIfSync]);

  const visibleGames = useMemo(() => {
    if (selection.type === "standings") return [];
    return games.filter((g) => gameBucket(g.status) === selection.bucket);
  }, [games, selection]);

  const visibleRaces = useMemo(() => {
    if (f1StandingsMode) return [];
    return f1Races.filter((r) => f1Bucket(r.status) === f1BucketSel);
  }, [f1Races, f1BucketSel, f1StandingsMode]);

  const activeMlbSection: MlbStandingSection | null = useMemo(() => {
    if (selection.type !== "standings" || !mlbStandings) return null;
    return mlbStandings.sections.find((s) => s.id === selection.sectionId) ?? mlbStandings.sections[0] ?? null;
  }, [selection, mlbStandings]);

  useEffect(() => {
    setActivePk((pk) => {
      if (pk && visibleGames.some((g) => g.id === pk)) return pk;
      return visibleGames[0]?.id ?? null;
    });
  }, [visibleGames]);

  useEffect(() => {
    setActiveMeetingKey((mk) => {
      if (mk && visibleRaces.some((r) => r.meetingKey === mk)) return mk;
      return visibleRaces[0]?.meetingKey ?? null;
    });
  }, [visibleRaces]);

  const selectedWeekend = useMemo(
    () => f1Races.find((r) => r.meetingKey === activeMeetingKey) ?? null,
    [f1Races, activeMeetingKey],
  );

  const weekendSessions = useMemo(() => {
    if (f1Detail?.sessions?.length) return f1Detail.sessions;
    return selectedWeekend?.sessions ?? [];
  }, [f1Detail, selectedWeekend]);

  useEffect(() => {
    if (!activeMeetingKey) {
      setActiveSessionKey(null);
      return;
    }
    const sessions = weekendSessions.length
      ? weekendSessions
      : selectedWeekend
        ? [
            {
              sessionKey: selectedWeekend.sessionKey,
              sessionName: "Race",
              kind: "race" as F1SessionKind,
              dateStart: selectedWeekend.dateStart,
              dateEnd: selectedWeekend.dateEnd,
              status: selectedWeekend.status,
            },
          ]
        : [];
    if (sessions.length === 0) {
      setActiveSessionKey(selectedWeekend?.sessionKey ?? null);
      return;
    }
    setF1SessionKind((prev) => {
      const available = F1_KIND_PILLS.filter((p) => sessionsForKind(sessions, p.id).length > 0);
      if (available.some((p) => p.id === prev)) return prev;
      return defaultKind(sessions);
    });
  }, [activeMeetingKey, weekendSessions, selectedWeekend]);

  useEffect(() => {
    const sessions = weekendSessions;
    if (sessions.length === 0) return;
    const inKind = sessionsForKind(sessions, f1SessionKind);
    if (inKind.length === 0) return;
    setActiveSessionKey((prev) => {
      if (prev && inKind.some((s) => s.sessionKey === prev)) return prev;
      const race = inKind.find((s) => s.kind === "race");
      return (race ?? inKind[inKind.length - 1]).sessionKey;
    });
  }, [weekendSessions, f1SessionKind]);

  const kindSessions = useMemo(
    () => sessionsForKind(weekendSessions, f1SessionKind),
    [weekendSessions, f1SessionKind],
  );

  const availableKindPills = useMemo(
    () => F1_KIND_PILLS.filter((p) => sessionsForKind(weekendSessions, p.id).length > 0),
    [weekendSessions],
  );

  useEffect(() => {
    void reloadMlbMeta().catch((e) => setError(e instanceof Error ? e.message : "Failed to load sports"));
    void reloadF1Meta().catch(() => {
      /* F1 years optional until expanded */
    });
  }, [reloadMlbMeta, reloadF1Meta]);

  useEffect(() => {
    void reloadMlbSchedule();
  }, [reloadMlbSchedule]);

  useEffect(() => {
    void reloadMlbStandings();
  }, [reloadMlbStandings]);

  useEffect(() => {
    void reloadF1Races();
  }, [reloadF1Races]);

  useEffect(() => {
    void reloadF1Standings();
  }, [reloadF1Standings]);

  useEffect(() => {
    if (activeSport !== "mlb" || mlbStandingsMode || !activePk) {
      if (activeSport !== "mlb" || mlbStandingsMode) setDetail(null);
      return;
    }
    const key = `mlb.game.${activePk}`;
    let cancelled = false;
    setDetail(null);
    beginFetch(key);
    void backend.sports
      .gameWatch(activePk)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load game");
      })
      .finally(() => {
        if (!cancelled) endFetchIfSync(key);
      });
    return () => {
      cancelled = true;
      void backend.sports.gameUnwatch(activePk);
    };
  }, [activePk, backend, activeSport, mlbStandingsMode, beginFetch, endFetchIfSync]);

  useEffect(() => {
    if (activeSport !== "f1" || f1StandingsMode || !activeSessionKey) {
      if (activeSport !== "f1" || f1StandingsMode) setF1Detail(null);
      return;
    }
    const key = `f1.race.${activeSessionKey}`;
    let cancelled = false;
    setF1Detail(null);
    beginFetch(key);
    void backend.sports
      .f1RaceWatch(activeSessionKey)
      .then((d) => {
        if (!cancelled) setF1Detail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load race");
      })
      .finally(() => {
        if (!cancelled) endFetchIfSync(key);
      });
    return () => {
      cancelled = true;
      void backend.sports.f1RaceUnwatch(activeSessionKey);
    };
  }, [activeSessionKey, backend, activeSport, f1StandingsMode, beginFetch, endFetchIfSync]);

  useEffect(() => {
    setScoringOnly(false);
    setMlbDetailTab("plays");
  }, [activePk]);

  useEffect(() => {
    setSignificantOnly(true);
    setF1DetailTab("classification");
  }, [activeSessionKey]);

  useEffect(() => {
    return backend.onEvent((ev) => {
      if (ev.event === "sports.refresh") {
        const payload = ev.payload as SportsRefreshEvent;
        if (!payload?.key) return;
        if (payload.phase === "started") {
          syncRefreshKeys.current.delete(payload.key);
          setKeyRefreshing(payload.key, true);
        } else {
          syncRefreshKeys.current.delete(payload.key);
          setKeyRefreshing(payload.key, false);
        }
        return;
      }
      if (ev.event === "sports.cache.updated") {
        const payload = ev.payload as SportsCacheUpdatedEvent;
        switch (payload.resource) {
          case "mlb.schedule":
            if (
              payload.season === season &&
              (payload.teamId ?? 0) === (scheduleTeamId ?? 0) &&
              Array.isArray(payload.games)
            ) {
              setGames(payload.games);
            } else if (Array.isArray(payload.data)) {
              // fallback raw data field
            }
            break;
          case "mlb.standings":
            if (payload.season === season && payload.standings && "sections" in payload.standings) {
              setMlbStandings(payload.standings as MlbStandings);
            }
            break;
          case "f1.races":
            if (payload.year === f1Year && Array.isArray(payload.races)) {
              setF1Races(payload.races);
            }
            break;
          case "f1.standings":
            if (payload.year === f1Year && payload.standings && "drivers" in payload.standings) {
              setF1Standings(payload.standings as F1Standings);
            }
            break;
          case "mlb.game": {
            const d = (payload.detail ?? payload.data) as MlbGameDetail | undefined;
            if (d?.game?.id === activePk) setDetail(d);
            break;
          }
          case "f1.race": {
            const d = (payload.detail ?? payload.data) as F1RaceDetail | undefined;
            if (d?.race?.sessionKey === activeSessionKey) setF1Detail(d);
            break;
          }
          case "mlb.teams":
          case "mlb.seasons":
          case "f1.years":
            break;
          default:
            break;
        }
        return;
      }
      if (ev.event === "sports.game.updated") {
        const payload = ev.payload as MlbGameDetail;
        if (payload?.game?.id === activePk) {
          setDetail(payload);
          setGames((prev) =>
            prev.map((g) => (g.id === payload.game.id ? { ...g, ...payload.game } : g)),
          );
        }
        return;
      }
      if (ev.event === "sports.f1.race.updated") {
        const payload = ev.payload as F1RaceDetail;
        if (payload?.race?.sessionKey === activeSessionKey) {
          setF1Detail(payload);
          setF1Races((prev) =>
            prev.map((r) =>
              r.sessionKey === payload.race.sessionKey ? { ...r, ...payload.race } : r,
            ),
          );
        }
      }
    });
  }, [
    backend,
    activePk,
    activeSessionKey,
    season,
    scheduleTeamId,
    f1Year,
    setKeyRefreshing,
  ]);

  const mlbSidebarRefreshing = isRefreshing("mlb.teams", "mlb.seasons", mlbScheduleKey, mlbStandingsKey);
  const f1SidebarRefreshing = isRefreshing("f1.years", f1RacesKey, f1StandingsKey);
  const mlbListRefreshing = mlbStandingsMode
    ? isRefreshing(mlbStandingsKey)
    : isRefreshing(mlbScheduleKey);
  const f1ListRefreshing = f1StandingsMode
    ? isRefreshing(f1StandingsKey)
    : isRefreshing(f1RacesKey);
  const mlbDetailRefreshing =
    !mlbStandingsMode && activePk != null && isRefreshing(`mlb.game.${activePk}`);
  const f1DetailRefreshing =
    !f1StandingsMode && activeSessionKey != null && isRefreshing(`f1.race.${activeSessionKey}`);
  const visiblePlays = useMemo(() => {
    if (!detail) return [];
    return scoringOnly ? detail.plays.filter((p) => p.isScoringPlay) : detail.plays;
  }, [detail, scoringOnly]);

  const visibleEvents = useMemo(() => {
    if (!f1Detail) return [];
    return significantOnly ? f1Detail.events.filter((e) => e.significant) : f1Detail.events;
  }, [f1Detail, significantOnly]);

  const gamesByDate = useMemo(() => {
    const map = new Map<string, MlbGame[]>();
    for (const g of visibleGames) {
      const key = g.officialDate || g.gameDate.slice(0, 10);
      const list = map.get(key) ?? [];
      list.push(g);
      map.set(key, list);
    }
    return [...map.entries()];
  }, [visibleGames]);

  const racesByDate = useMemo(() => {
    const map = new Map<string, F1Race[]>();
    for (const r of visibleRaces) {
      const key = r.dateStart ? r.dateStart.slice(0, 10) : "Unknown date";
      const list = map.get(key) ?? [];
      list.push(r);
      map.set(key, list);
    }
    return [...map.entries()];
  }, [visibleRaces]);

  const formatRaceDay = (isoDate: string) => {
    if (isoDate === "Unknown date") return isoDate;
    const d = new Date(`${isoDate}T12:00:00Z`);
    if (Number.isNaN(d.getTime())) return isoDate;
    return d.toLocaleDateString(undefined, {
      weekday: "short",
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const formatRaceTime = (dateStart: string) => {
    const d = new Date(dateStart);
    if (Number.isNaN(d.getTime())) return "";
    return d.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
  };

  return (
    <div className="layout">
      <aside className="pane sidebar">
        {SPORTS_REGISTRY.map((sport) => {
          const open = expandedSports.has(sport.id);
          return (
            <div key={sport.id} className="sports-league-section">
              <button
                type="button"
                className={`section-label sports-league-toggle ${open ? "open" : ""} ${
                  sport.available ? "" : "unavailable"
                }`}
                onClick={() => {
                  toggleSportExpanded(sport.id);
                  if (sport.available) setActiveSport(sport.id);
                }}
                aria-expanded={open}
              >
                <span>{open ? "▾" : "▸"}</span>
                <span>{sport.label}</span>
                {!sport.available ? <span className="sports-soon">Soon</span> : null}
                {sport.available &&
                ((sport.id === "mlb" && mlbSidebarRefreshing) ||
                  (sport.id === "f1" && f1SidebarRefreshing)) ? (
                  <SportsSpinner label="Updating" />
                ) : null}
              </button>

              {open && !sport.available && (
                <p className="sports-coming-soon muted">{sport.comingSoonNote ?? "Coming soon"}</p>
              )}

              {open && sport.available && sport.id === "mlb" && (
                <>
                  <div className="section-label sports-sublabel">Season</div>
                  <select
                    className="sports-season-select"
                    value={season ?? ""}
                    onChange={(e) => {
                      setActiveSport("mlb");
                      setSeason(Number(e.target.value));
                    }}
                  >
                    {seasons.map((s) => (
                      <option key={s.seasonId} value={s.seasonId}>
                        {s.seasonId}
                      </option>
                    ))}
                  </select>

                  <div className="sports-team-group">
                    <div className="nav-item sports-team-heading">
                      <span>League</span>
                    </div>
                    <button
                      type="button"
                      className={`nav-item nav-item-nested ${
                        activeSport === "mlb" && selection.type === "standings" ? "active" : ""
                      }`}
                      onClick={() =>
                        selectMlb({
                          type: "standings",
                          sectionId: mlbStandings?.sections[0]?.id ?? "div-201",
                        })
                      }
                    >
                      <span>Standings</span>
                    </button>
                  </div>

                  <div className="sports-team-group">
                    <div className="nav-item sports-team-heading">
                      <span>All followed</span>
                    </div>
                    {BUCKETS.map((b) => (
                      <button
                        key={`all-${b.id}`}
                        type="button"
                        className={`nav-item nav-item-nested ${
                          activeSport === "mlb" &&
                          selection.type === "all" &&
                          selection.bucket === b.id
                            ? "active"
                            : ""
                        }`}
                        onClick={() => selectMlb({ type: "all", bucket: b.id })}
                      >
                        <span>{b.label}</span>
                      </button>
                    ))}
                  </div>
                  {followedTeams.map((t) => (
                    <div key={t.id} className="sports-team-group">
                      <div className="nav-item sports-team-heading">
                        <span className="sports-team-row">
                          {t.logoUrl ? <img src={t.logoUrl} alt="" className="sports-logo" /> : null}
                          {t.abbreviation || t.name}
                        </span>
                      </div>
                      {BUCKETS.map((b) => (
                        <button
                          key={b.id}
                          type="button"
                          className={`nav-item nav-item-nested ${
                            activeSport === "mlb" &&
                            selection.type === "team" &&
                            selection.id === t.id &&
                            selection.bucket === b.id
                              ? "active"
                              : ""
                          }`}
                          onClick={() => selectMlb({ type: "team", id: t.id, bucket: b.id })}
                        >
                          <span>{b.label}</span>
                        </button>
                      ))}
                    </div>
                  ))}
                  {followedTeams.length === 0 && (
                    <div className="empty" style={{ height: "auto", padding: 12 }}>
                      Follow MLB teams in Settings → Sports
                    </div>
                  )}
                  <button type="button" className="nav-item" onClick={() => onOpenSettingsSports?.()}>
                    Manage baseball teams…
                  </button>
                </>
              )}

              {open && sport.available && sport.id === "f1" && (
                <>
                  <div className="section-label sports-sublabel">Season</div>
                  <select
                    className="sports-season-select"
                    value={f1Year ?? ""}
                    onChange={(e) => {
                      setActiveSport("f1");
                      setF1Year(Number(e.target.value));
                    }}
                  >
                    {f1Years.map((y) => (
                      <option key={y.year} value={y.year}>
                        {y.year}
                      </option>
                    ))}
                  </select>
                  <div className="sports-team-group">
                    <div className="nav-item sports-team-heading">
                      <span>League</span>
                    </div>
                    <button
                      type="button"
                      className={`nav-item nav-item-nested ${
                        activeSport === "f1" && f1Mode === "wdc" ? "active" : ""
                      }`}
                      onClick={() => selectF1Standings("wdc")}
                    >
                      <span>WDC</span>
                    </button>
                    <button
                      type="button"
                      className={`nav-item nav-item-nested ${
                        activeSport === "f1" && f1Mode === "wcc" ? "active" : ""
                      }`}
                      onClick={() => selectF1Standings("wcc")}
                    >
                      <span>WCC</span>
                    </button>
                  </div>
                  <div className="sports-team-group">
                    <div className="nav-item sports-team-heading">
                      <span>Races</span>
                    </div>
                    {BUCKETS.map((b) => (
                      <button
                        key={`f1-${b.id}`}
                        type="button"
                        className={`nav-item nav-item-nested ${
                          activeSport === "f1" && f1Mode === "races" && f1BucketSel === b.id
                            ? "active"
                            : ""
                        }`}
                        onClick={() => selectF1Races(b.id)}
                      >
                        <span>{b.label}</span>
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          );
        })}
      </aside>

      {activeSport === "mlb" ? (
        mlbStandingsMode ? (
          <>
            <section className="pane article-list">
              {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
              {mlbListRefreshing && !mlbStandings ? (
                <SportsLoadingPane label="Loading standings…" />
              ) : !mlbStandings || mlbStandings.sections.length === 0 ? (
                <div className="empty">
                  <h2>No standings</h2>
                  <p>Standings are not available for {season}.</p>
                </div>
              ) : (
                mlbStandings.sections.map((sec) => (
                  <button
                    key={sec.id}
                    type="button"
                    className={`article-row ${sec.id === activeMlbSection?.id ? "active" : ""}`}
                    onClick={() => selectMlb({ type: "standings", sectionId: sec.id })}
                  >
                    <div className="article-meta">
                      <span>{sec.league}</span>
                      <span>{sec.kind === "wildcard" ? "WC" : "Div"}</span>
                    </div>
                    <h3 className="article-title">
                      {sec.league} {sec.name}
                    </h3>
                    <p className="article-summary">{sec.teams.length} teams</p>
                  </button>
                ))
              )}
            </section>
            <section className="pane reader-pane">
              {!activeMlbSection ? (
                <div className="empty">
                  <h2>Standings</h2>
                  <p>Select a division or wild card race.</p>
                </div>
              ) : (
                <article className="reader sports-reader">
                  <div className="reader-kicker">
                    {season} · {activeMlbSection.kind === "wildcard" ? "Wild Card" : "Division"}
                  </div>
                  <h1 className="sports-scoreline" style={{ fontSize: "1.35rem" }}>
                    {activeMlbSection.league} {activeMlbSection.name}
                  </h1>
                  <div className="sports-linescore-wrap">
                    <table className="sports-linescore sports-standings-table">
                      <thead>
                        <tr>
                          <th>#</th>
                          <th>Team</th>
                          <th>W</th>
                          <th>L</th>
                          <th>PCT</th>
                          <th>GB</th>
                          {activeMlbSection.kind === "wildcard" ? <th>WCGB</th> : null}
                          <th>Diff</th>
                          <th>Strk</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activeMlbSection.teams.map((row) => (
                          <tr key={row.team.id}>
                            <td>{row.rank}</td>
                            <td className="sports-standings-team">
                              {row.team.logoUrl ? (
                                <img src={row.team.logoUrl} alt="" className="sports-logo" />
                              ) : null}
                              {row.team.abbreviation || row.team.name}
                              {row.clinched ? (
                                <span className="sports-scoring-badge" style={{ marginLeft: 6 }}>
                                  Clinched
                                </span>
                              ) : null}
                            </td>
                            <td>{row.wins}</td>
                            <td>{row.losses}</td>
                            <td>{row.winningPercentage || "—"}</td>
                            <td>{row.gamesBack || "—"}</td>
                            {activeMlbSection.kind === "wildcard" ? (
                              <td>{row.wildCardGamesBack || "—"}</td>
                            ) : null}
                            <td>
                              {row.runDifferential > 0 ? "+" : ""}
                              {row.runDifferential}
                            </td>
                            <td>{row.streak || "—"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </article>
              )}
            </section>
          </>
        ) : (
        <>
          <section className="pane article-list">
            {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
            {mlbListRefreshing ? (
              <SportsLoadingPane label="Loading schedule…" />
            ) : visibleGames.length === 0 ? (
              <div className="empty">
                <h2>No games</h2>
                <p>
                  {`No ${
                    selection.type === "all" || selection.type === "team"
                      ? BUCKETS.find((b) => b.id === selection.bucket)?.label.toLowerCase() ?? ""
                      : ""
                  } games${selection.type === "team" ? " for this team" : ""} in ${season}.`}
                </p>
              </div>
            ) : (
              gamesByDate.map(([date, dayGames]) => (
                <div key={date}>
                  <div className="section-label" style={{ padding: "8px 12px" }}>
                    {date}
                  </div>
                  {dayGames.map((g) => (
                    <button
                      key={g.id}
                      type="button"
                      className={`article-row ${g.id === activePk ? "active" : ""}`}
                      onClick={() => setActivePk(g.id)}
                    >
                      <div className="article-meta">
                        <span>{statusLabel(g.status, g.statusDetail)}</span>
                        <span>{scoreText(g)}</span>
                      </div>
                      <h3 className="article-title">{matchup(g)}</h3>
                      <p className="article-summary">
                        {g.currentInning
                          ? `${g.currentInningHalf === "top" ? "Top" : "Bot"} ${g.currentInning}`
                          : new Date(g.gameDate).toLocaleString()}
                      </p>
                    </button>
                  ))}
                </div>
              ))
            )}
          </section>

          <section className="pane reader-pane">
            {!detail ? (
              activePk != null || mlbDetailRefreshing ? (
                <SportsLoadingPane label="Loading game…" />
              ) : (
                <div className="empty">
                  <h2>Sports</h2>
                  <p>Select a game to see the score, linescore, and plays.</p>
                </div>
              )
            ) : (
              <article className="reader sports-reader">
                <div className="reader-kicker">
                  {statusLabel(detail.game.status, detail.game.statusDetail)}
                  {detail.game.currentInning
                    ? ` · ${detail.game.currentInningHalf === "top" ? "Top" : "Bot"} ${detail.game.currentInning}`
                    : ""}
                </div>
                <h1 className="sports-scoreline">
                  <span>
                    {detail.game.awayTeam.abbreviation || detail.game.awayTeam.name}{" "}
                    <strong>{detail.game.awayScore ?? "—"}</strong>
                  </span>
                  <span className="muted">@</span>
                  <span>
                    {detail.game.homeTeam.abbreviation || detail.game.homeTeam.name}{" "}
                    <strong>{detail.game.homeScore ?? "—"}</strong>
                  </span>
                </h1>

                {detail.innings.length > 0 && (
                  <div className="sports-linescore-wrap">
                    <table className="sports-linescore">
                      <thead>
                        <tr>
                          <th />
                          {detail.innings.map((inn) => (
                            <th key={inn.number}>{inn.number}</th>
                          ))}
                          <th>R</th>
                          <th>H</th>
                          <th>E</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr>
                          <th>{detail.game.awayTeam.abbreviation || "AWY"}</th>
                          {detail.innings.map((inn) => (
                            <td key={inn.number}>{inn.awayRuns}</td>
                          ))}
                          <td>{detail.game.awayScore ?? 0}</td>
                          <td>{detail.awayHits ?? "—"}</td>
                          <td>{detail.awayErrors ?? "—"}</td>
                        </tr>
                        <tr>
                          <th>{detail.game.homeTeam.abbreviation || "HME"}</th>
                          {detail.innings.map((inn) => (
                            <td key={inn.number}>{inn.homeRuns}</td>
                          ))}
                          <td>{detail.game.homeScore ?? 0}</td>
                          <td>{detail.homeHits ?? "—"}</td>
                          <td>{detail.homeErrors ?? "—"}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                )}

                <div className="sports-plays-header">
                  <div className="content-tabs sports-plays-filter" role="group" aria-label="Game detail view">
                    <button
                      type="button"
                      className={`content-tab ${mlbDetailTab === "plays" ? "active" : ""}`}
                      onClick={() => setMlbDetailTab("plays")}
                    >
                      Plays
                    </button>
                    <button
                      type="button"
                      className={`content-tab ${mlbDetailTab === "stats" ? "active" : ""}`}
                      onClick={() => setMlbDetailTab("stats")}
                    >
                      Player Stats
                    </button>
                  </div>
                  {mlbDetailTab === "plays" ? (
                    <div className="content-tabs sports-plays-filter" role="group" aria-label="Play filter">
                      <button
                        type="button"
                        className={`content-tab ${!scoringOnly ? "active" : ""}`}
                        onClick={() => setScoringOnly(false)}
                      >
                        All
                      </button>
                      <button
                        type="button"
                        className={`content-tab ${scoringOnly ? "active" : ""}`}
                        onClick={() => setScoringOnly(true)}
                      >
                        Scoring
                      </button>
                    </div>
                  ) : null}
                </div>

                {mlbDetailTab === "plays" ? (
                  <div className="sports-plays">
                    {detail.plays.length === 0 ? (
                      <p className="muted">No plays yet.</p>
                    ) : visiblePlays.length === 0 ? (
                      <p className="muted">No scoring plays yet.</p>
                    ) : (
                      visiblePlays.map((p) => (
                        <div
                          key={p.id}
                          className={`sports-play ${p.isScoringPlay ? "scoring" : ""}`}
                        >
                          <div className="sports-play-meta">
                            <span>
                              {p.half === "top" ? "Top" : "Bot"} {p.inning}
                            </span>
                            <span>{p.event}</span>
                            {p.isScoringPlay ? (
                              <span className="sports-scoring-badge">Scoring</span>
                            ) : null}
                            {p.awayScore != null && p.homeScore != null ? (
                              <span>
                                {p.awayScore}–{p.homeScore}
                              </span>
                            ) : null}
                          </div>
                          <p>{p.description}</p>
                        </div>
                      ))
                    )}
                  </div>
                ) : (
                  <div className="sports-player-stats">
                    {!detail.awayBox && !detail.homeBox ? (
                      <p className="muted">Player stats are not available yet for this game.</p>
                    ) : (
                      <>
                        {detail.awayBox ? <TeamBoxSection box={detail.awayBox} /> : null}
                        {detail.homeBox ? <TeamBoxSection box={detail.homeBox} /> : null}
                      </>
                    )}
                  </div>
                )}
              </article>
            )}
          </section>
        </>
        )
      ) : activeSport === "f1" ? (
        f1StandingsMode ? (
          <>
            <section className="pane article-list">
              {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
              {f1ListRefreshing && !f1Standings ? (
                <SportsLoadingPane label="Loading championship…" />
              ) : f1Mode === "wdc" ? (
                !(f1Standings?.drivers.length) ? (
                  <div className="empty">
                    <h2>No WDC data</h2>
                    <p>Driver standings unavailable for {f1Year}.</p>
                  </div>
                ) : (
                  f1Standings.drivers.map((d) => (
                    <div key={d.driverNumber} className="article-row">
                      <div className="article-meta">
                        <span>P{d.position}</span>
                        <span>{d.points} pts</span>
                      </div>
                      <h3 className="article-title">{d.nameAcronym || d.name}</h3>
                      <p className="article-summary">{d.teamName || d.name}</p>
                    </div>
                  ))
                )
              ) : !(f1Standings?.constructors.length) ? (
                <div className="empty">
                  <h2>No WCC data</h2>
                  <p>Constructor standings unavailable for {f1Year}.</p>
                </div>
              ) : (
                f1Standings.constructors.map((t) => (
                  <div key={t.teamName} className="article-row">
                    <div className="article-meta">
                      <span>P{t.position}</span>
                      <span>{t.points} pts</span>
                    </div>
                    <h3 className="article-title">{t.teamName}</h3>
                  </div>
                ))
              )}
            </section>
            <section className="pane reader-pane">
              <article className="reader sports-reader">
                <div className="reader-kicker">
                  {f1Year} championship
                  {f1Standings?.meetingName ? ` · after ${f1Standings.meetingName}` : ""}
                </div>
                <h1 className="sports-scoreline" style={{ fontSize: "1.35rem" }}>
                  {f1Mode === "wdc" ? "Drivers’ Championship" : "Constructors’ Championship"}
                </h1>
                <div className="sports-linescore-wrap">
                  {f1Mode === "wdc" ? (
                    <table className="sports-linescore sports-standings-table sports-f1-results">
                      <thead>
                        <tr>
                          <th>Pos</th>
                          <th>Driver</th>
                          <th>Team</th>
                          <th>Pts</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(f1Standings?.drivers ?? []).map((d) => (
                          <tr key={d.driverNumber}>
                            <td>{d.position}</td>
                            <td>{d.nameAcronym || d.name}</td>
                            <td>{d.teamName || "—"}</td>
                            <td>{d.points}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  ) : (
                    <table className="sports-linescore sports-standings-table sports-f1-results">
                      <thead>
                        <tr>
                          <th>Pos</th>
                          <th>Team</th>
                          <th>Pts</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(f1Standings?.constructors ?? []).map((t) => (
                          <tr key={t.teamName}>
                            <td>{t.position}</td>
                            <td>{t.teamName}</td>
                            <td>{t.points}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              </article>
            </section>
          </>
        ) : (
        <>
          <section className="pane article-list">
            {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
            {f1ListRefreshing ? (
              <SportsLoadingPane label="Loading races…" />
            ) : visibleRaces.length === 0 ? (
              <div className="empty">
                <h2>No races</h2>
                <p>
                  {`No ${BUCKETS.find((b) => b.id === f1BucketSel)?.label.toLowerCase() ?? ""} races in ${f1Year}.`}
                </p>
              </div>
            ) : (
              racesByDate.map(([date, dayRaces]) => (
                <div key={date}>
                  <div className="section-label" style={{ padding: "8px 12px" }}>
                    {formatRaceDay(date)}
                  </div>
                  {dayRaces.map((r) => (
                    <button
                      key={r.meetingKey}
                      type="button"
                      className={`article-row ${r.meetingKey === activeMeetingKey ? "active" : ""}`}
                      onClick={() => {
                        setActiveMeetingKey(r.meetingKey);
                        setActiveSessionKey(r.sessionKey);
                        setF1SessionKind(defaultKind(r.sessions ?? []));
                      }}
                    >
                      <div className="article-meta">
                        <span>{f1StatusLabel(r.status)}</span>
                        <span>{r.dateStart ? formatRaceTime(r.dateStart) : r.countryCode || r.countryName}</span>
                      </div>
                      <h3 className="article-title">{r.name}</h3>
                      <p className="article-summary">
                        {r.circuitShortName || r.location}
                        {r.countryName ? ` · ${r.countryName}` : ""}
                      </p>
                    </button>
                  ))}
                </div>
              ))
            )}
          </section>

          <section className="pane reader-pane">
            {!f1Detail ? (
              activeSessionKey != null || f1DetailRefreshing ? (
                <SportsLoadingPane label="Loading race…" />
              ) : (
                <div className="empty">
                  <h2>F1</h2>
                  <p>Select a race to see classification and race-control events.</p>
                </div>
              )
            ) : (
              <article className="reader sports-reader">
                <div className="reader-kicker">
                  {f1StatusLabel(f1Detail.session?.status ?? f1Detail.race.status)}
                  {f1Detail.session?.sessionName
                    ? ` · ${f1Detail.session.sessionName}`
                    : ""}
                  {f1Detail.race.countryName ? ` · ${f1Detail.race.countryName}` : ""}
                </div>
                <h1 className="sports-scoreline" style={{ fontSize: "1.35rem" }}>
                  {f1Detail.race.name}
                </h1>
                <p className="muted" style={{ marginTop: -8, marginBottom: 12 }}>
                  {f1Detail.race.circuitShortName || f1Detail.race.location}
                  {f1Detail.session?.dateStart
                    ? ` · ${new Date(f1Detail.session.dateStart).toLocaleString()}`
                    : f1Detail.race.dateStart
                      ? ` · ${new Date(f1Detail.race.dateStart).toLocaleString()}`
                      : ""}
                </p>

                {availableKindPills.length > 0 ? (
                  <div
                    className="content-tabs sports-plays-filter"
                    role="group"
                    aria-label="Session type"
                    style={{ marginBottom: 8 }}
                  >
                    {availableKindPills.map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        className={`content-tab ${f1SessionKind === p.id ? "active" : ""}`}
                        onClick={() => setF1SessionKind(p.id)}
                      >
                        {p.label}
                      </button>
                    ))}
                  </div>
                ) : null}

                {kindSessions.length > 1 ? (
                  <div
                    className="content-tabs sports-plays-filter"
                    role="group"
                    aria-label="Session"
                    style={{ marginBottom: 12 }}
                  >
                    {kindSessions.map((s) => (
                      <button
                        key={s.sessionKey}
                        type="button"
                        className={`content-tab ${s.sessionKey === activeSessionKey ? "active" : ""}`}
                        onClick={() => setActiveSessionKey(s.sessionKey)}
                      >
                        {s.sessionName}
                      </button>
                    ))}
                  </div>
                ) : null}

                <div className="sports-plays-header">
                  <div
                    className="content-tabs sports-plays-filter"
                    role="group"
                    aria-label="Session detail view"
                  >
                    <button
                      type="button"
                      className={`content-tab ${f1DetailTab === "classification" ? "active" : ""}`}
                      onClick={() => setF1DetailTab("classification")}
                    >
                      Classification
                    </button>
                    <button
                      type="button"
                      className={`content-tab ${f1DetailTab === "events" ? "active" : ""}`}
                      onClick={() => setF1DetailTab("events")}
                    >
                      Events
                    </button>
                  </div>
                  {f1DetailTab === "events" ? (
                    <div
                      className="content-tabs sports-plays-filter"
                      role="group"
                      aria-label="Event filter"
                    >
                      <button
                        type="button"
                        className={`content-tab ${!significantOnly ? "active" : ""}`}
                        onClick={() => setSignificantOnly(false)}
                      >
                        All
                      </button>
                      <button
                        type="button"
                        className={`content-tab ${significantOnly ? "active" : ""}`}
                        onClick={() => setSignificantOnly(true)}
                      >
                        Significant
                      </button>
                    </div>
                  ) : null}
                </div>

                {f1DetailTab === "classification" ? (
                  f1Detail.results.length === 0 ? (
                    <p className="muted">No classification yet.</p>
                  ) : (
                    <div className="sports-linescore-wrap">
                      <table className="sports-linescore sports-f1-results">
                        <thead>
                          <tr>
                            <th>Pos</th>
                            <th>Driver</th>
                            <th>Team</th>
                            <th>Pts</th>
                            <th>Gap</th>
                          </tr>
                        </thead>
                        <tbody>
                          {f1Detail.results.map((r) => (
                            <tr key={r.driverNumber}>
                              <td>{r.position}</td>
                              <td>
                                {r.nameAcronym || r.name}
                                {r.dnf || r.dns || r.dsq ? (
                                  <span className="sports-scoring-badge" style={{ marginLeft: 6 }}>
                                    {resultStatus(r) || "OUT"}
                                  </span>
                                ) : null}
                              </td>
                              <td>{r.teamName || "—"}</td>
                              <td>{r.points}</td>
                              <td>{resultStatus(r) || "—"}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )
                ) : (
                  <div className="sports-plays">
                    {f1Detail.events.length === 0 ? (
                      <p className="muted">No race-control messages.</p>
                    ) : visibleEvents.length === 0 ? (
                      <p className="muted">No significant events.</p>
                    ) : (
                      visibleEvents.map((e) => (
                        <div
                          key={e.id}
                          className={`sports-play ${e.significant ? "scoring" : ""}`}
                        >
                          <div className="sports-play-meta">
                            {e.lapNumber != null ? <span>Lap {e.lapNumber}</span> : null}
                            <span>
                              {e.category}
                              {e.flag ? ` · ${e.flag}` : ""}
                            </span>
                            {e.significant ? (
                              <span className="sports-scoring-badge">Significant</span>
                            ) : null}
                            <span>{e.date ? new Date(e.date).toLocaleTimeString() : ""}</span>
                          </div>
                          <p>
                            {e.message}
                            {e.driverName ? ` (${e.driverName})` : ""}
                          </p>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </article>
            )}
          </section>
        </>
        )
      ) : (
        <section className="pane article-list">
          <div className="empty">
            <h2>{SPORTS_REGISTRY.find((s) => s.id === activeSport)?.label ?? "Sports"}</h2>
            <p>Coming soon.</p>
          </div>
        </section>
      )}
    </div>
  );
}
