import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  F1Race,
  F1RaceDetail,
  F1Season,
  F1Session,
  F1SessionKind,
  F1Standings,
  MlbGame,
  MlbGameDetail,
  MlbRoster,
  MlbSeason,
  MlbStandings,
  MlbTeam,
  ReaderBackend,
  SportsCacheUpdatedEvent,
  SportsRefreshEvent,
} from "@rss-reader/shared";
import { SportsSpinner } from "../components/SportsSpinner";
import { BaseballTeamDashboard } from "../components/BaseballTeamDashboard";
import { BaseballStandingsDashboard } from "../components/BaseballStandingsDashboard";
import { F1Dashboard } from "../components/F1Dashboard";
import { localDateKey, preferredGameId, shiftLocalDateKey } from "../lib/baseballDashboard";
import { preferredRaceMeetingKey } from "../lib/f1Dashboard";
import { mlbTeamHref, mlbTeamRouteFromHash } from "../lib/sportsDeepLinks";
import { SPORTS_REGISTRY, type SportId } from "../lib/sportsRegistry";
import { GENERIC_ERROR_MESSAGE } from "../lib/errors";

type Props = {
  backend: ReaderBackend;
  onOpenSettingsSports?: () => void;
};

const F1_KIND_PILLS: { id: F1SessionKind; label: string }[] = [
  { id: "practice", label: "Practice" },
  { id: "sprint_quali", label: "Sprint Quali" },
  { id: "sprint", label: "Sprint" },
  { id: "quali", label: "Quali" },
  { id: "race", label: "Race" },
];

function sessionMatchesKind(session: F1Session, kind: F1SessionKind): boolean {
  return session.kind === kind;
}

function sessionsForKind(sessions: F1Session[], kind: F1SessionKind): F1Session[] {
  return sessions.filter((s) => sessionMatchesKind(s, kind));
}

function defaultKind(sessions: F1Session[]): F1SessionKind {
  if (sessions.some((s) => s.kind === "race")) return "race";
  if (sessions.some((s) => s.kind === "sprint")) return "sprint";
  if (sessions.some((s) => s.kind === "quali")) return "quali";
  if (sessions.some((s) => s.kind === "sprint_quali")) return "sprint_quali";
  if (sessions.some((s) => s.kind === "practice")) return "practice";
  return "race";
}

export function SportsView({ backend, onOpenSettingsSports }: Props) {
  const initialMlbRoute = mlbTeamRouteFromHash(window.location.hash);
  const [activeSport, setActiveSport] = useState<SportId>("mlb");
  const [mlbView, setMlbView] = useState<"standings" | "team">(
    initialMlbRoute ? "team" : "standings",
  );

  // --- MLB ---
  const [teams, setTeams] = useState<MlbTeam[]>([]);
  const [followed, setFollowed] = useState<number[]>([]);
  const [seasons, setSeasons] = useState<MlbSeason[]>([]);
  const [season, setSeason] = useState<number | null>(initialMlbRoute?.season ?? null);
  const [selectedMlbTeamId, setSelectedMlbTeamId] = useState<number | null>(
    initialMlbRoute?.teamId ?? null,
  );
  const [pinningMlbTeamId, setPinningMlbTeamId] = useState<number | null>(null);
  const [games, setGames] = useState<MlbGame[]>([]);
  const [activePk, setActivePk] = useState<number | null>(null);
  const [detail, setDetail] = useState<MlbGameDetail | null>(null);
  const [mlbStandings, setMlbStandings] = useState<MlbStandings | null>(null);
  const [dailyGames, setDailyGames] = useState<MlbGame[]>([]);
  const [mlbSlateDate, setMlbSlateDate] = useState(() => localDateKey());
  const [roster, setRoster] = useState<MlbRoster | null>(null);

  // --- F1 ---
  const [f1Years, setF1Years] = useState<F1Season[]>([]);
  const [f1Year, setF1Year] = useState<number | null>(null);
  const [f1Races, setF1Races] = useState<F1Race[]>([]);
  const [activeMeetingKey, setActiveMeetingKey] = useState<number | null>(null);
  const [activeSessionKey, setActiveSessionKey] = useState<number | null>(null);
  const [f1SessionKind, setF1SessionKind] = useState<F1SessionKind>("race");
  const [f1Detail, setF1Detail] = useState<F1RaceDetail | null>(null);
  const [f1Standings, setF1Standings] = useState<F1Standings | null>(null);

  const [refreshingKeys, setRefreshingKeys] = useState<ReadonlySet<string>>(() => new Set());
  const syncRefreshKeys = useRef(new Set<string>());
  const dailyScheduleRequest = useRef(0);
  const pendingMlbGameId = useRef<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expandedSports, setExpandedSports] = useState<ReadonlySet<SportId>>(
    () => new Set<SportId>(SPORTS_REGISTRY.filter((sport) => sport.available).map((sport) => sport.id)),
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

  const selectMlbTeam = (teamId: number) => {
    pendingMlbGameId.current = null;
    setActiveSport("mlb");
    setMlbView("team");
    setSelectedMlbTeamId(teamId);
    setExpandedSports((prev) => new Set(prev).add("mlb"));
    const href = mlbTeamHref(teamId, season);
    if (window.location.hash !== href) window.location.hash = href;
  };

  const selectMlbGame = (game: MlbGame) => {
    const targetSeason = game.season || season;
    pendingMlbGameId.current = game.id;
    setActiveSport("mlb");
    setMlbView("team");
    if (targetSeason != null) setSeason(targetSeason);
    setSelectedMlbTeamId(game.homeTeam.id);
    setExpandedSports((prev) => new Set(prev).add("mlb"));
    const href = mlbTeamHref(game.homeTeam.id, targetSeason);
    if (window.location.hash !== href) window.location.hash = href;
  };

  const selectMlbStandings = () => {
    pendingMlbGameId.current = null;
    setActiveSport("mlb");
    setMlbView("standings");
    setMlbSlateDate(localDateKey());
    setExpandedSports((prev) => new Set(prev).add("mlb"));
    if (window.location.hash.startsWith("#sports/mlb/team/")) {
      window.history.replaceState(null, "", "#sports/mlb/standings");
    }
  };

  const selectF1Standings = () => {
    setActiveSport("f1");
    setExpandedSports((prev) => new Set(prev).add("f1"));
    if (window.location.hash.startsWith("#sports/mlb/")) {
      window.history.replaceState(null, "", "#sports/f1/standings");
    }
    requestAnimationFrame(() => {
      document.getElementById("f1-standings")?.scrollIntoView({
        behavior: "smooth",
        block: "start",
        inline: "nearest",
      });
    });
  };

  const pinMlbTeam = async (teamId: number) => {
    if (followed.includes(teamId) || pinningMlbTeamId != null) return;
    setPinningMlbTeamId(teamId);
    setError(null);
    try {
      const ids = await backend.sports.followedSet([...followed, teamId]);
      setFollowed(ids ?? []);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      setPinningMlbTeamId(null);
    }
  };

  const followedTeams = useMemo(
    () => teams.filter((t) => followed.includes(t.id)).sort((a, b) => a.name.localeCompare(b.name)),
    [teams, followed],
  );
  const selectedMlbTeam = useMemo(
    () => teams.find((team) => team.id === selectedMlbTeamId) ?? null,
    [teams, selectedMlbTeamId],
  );
  const temporaryMlbTeam = useMemo(
    () =>
      selectedMlbTeam && !followed.includes(selectedMlbTeam.id) ? selectedMlbTeam : null,
    [followed, selectedMlbTeam],
  );

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
      setSeason((prev) =>
        prev != null && (s ?? []).some((item) => item.seasonId === prev)
          ? prev
          : (s?.[0]?.seasonId ?? new Date().getFullYear()),
      );
      setSelectedMlbTeamId((prev) =>
        prev != null && (t ?? []).some((team) => team.id === prev) ? prev : (f?.[0] ?? null),
      );
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

  const scheduleTeamId = selectedMlbTeamId ?? undefined;

  const mlbScheduleKey =
    season == null ? "" : `mlb.schedule.${season}.${scheduleTeamId ?? 0}`;
  const mlbStandingsKey = season == null ? "" : `mlb.standings.${season}`;
  const todayKey = localDateKey();
  const mlbDailyScheduleKey = `mlb.schedule.daily.v2.${mlbSlateDate}`;
  const mlbRosterKey = season == null || selectedMlbTeamId == null
    ? ""
    : `mlb.roster.${selectedMlbTeamId}.${season}`;
  const f1RacesKey = f1Year == null ? "" : `f1.races.${f1Year}`;
  const f1StandingsKey = f1Year == null ? "" : `f1.standings.${f1Year}`;

  const reloadMlbSchedule = useCallback(async () => {
    if (season == null || activeSport !== "mlb" || mlbView !== "team" || scheduleTeamId == null || !mlbScheduleKey) return;
    beginFetch(mlbScheduleKey);
    setError(null);
    setGames([]);
    setActivePk(null);
    setDetail(null);
    try {
      const list = await backend.sports.schedule({ teamId: scheduleTeamId, season });
      const nextGames = list ?? [];
      setGames(nextGames);
      const requestedGameId = pendingMlbGameId.current;
      if (requestedGameId != null && nextGames.some((game) => game.id === requestedGameId)) {
        pendingMlbGameId.current = null;
        setActivePk(requestedGameId);
      }
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      endFetchIfSync(mlbScheduleKey);
    }
  }, [
    backend,
    season,
    scheduleTeamId,
    activeSport,
    mlbView,
    mlbScheduleKey,
    beginFetch,
    endFetchIfSync,
  ]);

  const reloadMlbStandings = useCallback(async () => {
    if (season == null || activeSport !== "mlb" || !mlbStandingsKey) return;
    beginFetch(mlbStandingsKey);
    setError(null);
    setMlbStandings(null);
    try {
      const data = await backend.sports.standings({ season });
      setMlbStandings(data);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      endFetchIfSync(mlbStandingsKey);
    }
  }, [backend, season, activeSport, mlbStandingsKey, beginFetch, endFetchIfSync]);

  const reloadMlbDailySchedule = useCallback(async () => {
    if (activeSport !== "mlb" || mlbView !== "standings") return;
    const requestId = ++dailyScheduleRequest.current;
    beginFetch(mlbDailyScheduleKey);
    setError(null);
    setDailyGames([]);
    try {
      const list = await backend.sports.dailySchedule({ date: mlbSlateDate });
      if (dailyScheduleRequest.current === requestId) setDailyGames(list ?? []);
    } catch (e) {
      if (dailyScheduleRequest.current === requestId) {
        setError(GENERIC_ERROR_MESSAGE);
      }
    } finally {
      endFetchIfSync(mlbDailyScheduleKey);
    }
  }, [activeSport, backend, beginFetch, endFetchIfSync, mlbDailyScheduleKey, mlbSlateDate, mlbView]);

  const reloadMlbRoster = useCallback(async () => {
    if (
      activeSport !== "mlb" || mlbView !== "team" || season == null ||
      selectedMlbTeamId == null || !mlbRosterKey
    ) return;
    beginFetch(mlbRosterKey);
    setError(null);
    setRoster(null);
    try {
      const data = await backend.sports.roster({ teamId: selectedMlbTeamId, season });
      setRoster(data);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      endFetchIfSync(mlbRosterKey);
    }
  }, [activeSport, backend, beginFetch, endFetchIfSync, mlbRosterKey, mlbView, season, selectedMlbTeamId]);

  const reloadF1Races = useCallback(async () => {
    if (f1Year == null || activeSport !== "f1" || !f1RacesKey) return;
    beginFetch(f1RacesKey);
    setError(null);
    setF1Races([]);
    setActiveSessionKey(null);
    setF1Detail(null);
    try {
      const list = await backend.sports.f1Races({ year: f1Year });
      setF1Races(list ?? []);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      endFetchIfSync(f1RacesKey);
    }
  }, [backend, f1Year, activeSport, f1RacesKey, beginFetch, endFetchIfSync]);

  const reloadF1Standings = useCallback(async () => {
    if (f1Year == null || activeSport !== "f1" || !f1StandingsKey) return;
    beginFetch(f1StandingsKey);
    setError(null);
    setF1Standings(null);
    try {
      const data = await backend.sports.f1Standings({ year: f1Year });
      setF1Standings(data);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      endFetchIfSync(f1StandingsKey);
    }
  }, [backend, f1Year, activeSport, f1StandingsKey, beginFetch, endFetchIfSync]);

  const visibleRaces = useMemo(() => {
    return f1Races;
  }, [f1Races]);

  useEffect(() => {
    setActivePk((pk) => {
      if (pk && games.some((g) => g.id === pk)) return pk;
      return preferredGameId(games);
    });
  }, [games]);

  useEffect(() => {
    setActiveMeetingKey((mk) => {
      if (mk && visibleRaces.some((r) => r.meetingKey === mk)) return mk;
      return preferredRaceMeetingKey(visibleRaces);
    });
  }, [visibleRaces]);

  const selectedWeekend = useMemo(
    () => f1Races.find((r) => r.meetingKey === activeMeetingKey) ?? null,
    [f1Races, activeMeetingKey],
  );

  const weekendSessions = useMemo(() => {
    const detailSessions = f1Detail?.sessions;
    if (
      f1Detail?.race.meetingKey === selectedWeekend?.meetingKey &&
      detailSessions?.length
    ) {
      return detailSessions;
    }
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
    void reloadMlbMeta().catch(() => setError(GENERIC_ERROR_MESSAGE));
    void reloadF1Meta().catch(() => {
      /* F1 years optional until expanded */
    });
  }, [reloadMlbMeta, reloadF1Meta]);

  useEffect(() => {
    const applyMlbDeepLink = () => {
      const route = mlbTeamRouteFromHash(window.location.hash);
      if (!route) return;
      setActiveSport("mlb");
      setMlbView("team");
      setExpandedSports((prev) => new Set(prev).add("mlb"));
      setSelectedMlbTeamId(route.teamId);
      if (route.season != null) setSeason(route.season);
    };
    window.addEventListener("hashchange", applyMlbDeepLink);
    return () => window.removeEventListener("hashchange", applyMlbDeepLink);
  }, []);

  useEffect(() => {
    if (activeSport !== "mlb" || mlbView !== "team" || selectedMlbTeamId == null) return;
    const href = mlbTeamHref(selectedMlbTeamId, season);
    if (window.location.hash !== href) window.history.replaceState(null, "", href);
  }, [activeSport, mlbView, season, selectedMlbTeamId]);

  useEffect(() => {
    void reloadMlbSchedule();
  }, [reloadMlbSchedule]);

  useEffect(() => {
    void reloadMlbStandings();
  }, [reloadMlbStandings]);

  useEffect(() => {
    void reloadMlbDailySchedule();
  }, [reloadMlbDailySchedule]);

  useEffect(() => {
    void reloadMlbRoster();
  }, [reloadMlbRoster]);

  useEffect(() => {
    void reloadF1Races();
  }, [reloadF1Races]);

  useEffect(() => {
    void reloadF1Standings();
  }, [reloadF1Standings]);

  useEffect(() => {
    if (activeSport !== "mlb" || mlbView !== "team" || !activePk) {
      if (activeSport !== "mlb") setDetail(null);
      return;
    }
    const key = `mlb.game.v2.${activePk}`;
    let cancelled = false;
    setDetail(null);
    beginFetch(key);
    void backend.sports
      .gameWatch(activePk)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(GENERIC_ERROR_MESSAGE);
      })
      .finally(() => {
        if (!cancelled) endFetchIfSync(key);
      });
    return () => {
      cancelled = true;
      void backend.sports.gameUnwatch(activePk);
    };
  }, [activePk, backend, activeSport, beginFetch, endFetchIfSync, mlbView]);

  useEffect(() => {
    if (activeSport !== "f1" || !activeSessionKey) {
      if (activeSport !== "f1") setF1Detail(null);
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
        if (!cancelled) setError(GENERIC_ERROR_MESSAGE);
      })
      .finally(() => {
        if (!cancelled) endFetchIfSync(key);
      });
    return () => {
      cancelled = true;
      void backend.sports.f1RaceUnwatch(activeSessionKey);
    };
  }, [activeSessionKey, backend, activeSport, beginFetch, endFetchIfSync]);

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
          case "mlb.schedule.daily":
            if (payload.date === mlbSlateDate && Array.isArray(payload.games)) {
              setDailyGames(payload.games);
            }
            break;
          case "mlb.standings":
            if (payload.season === season && payload.standings && "sections" in payload.standings) {
              setMlbStandings(payload.standings as MlbStandings);
            }
            break;
          case "mlb.roster":
            if (
              payload.season === season && payload.teamId === selectedMlbTeamId &&
              payload.roster
            ) {
              setRoster(payload.roster);
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
            if (d && (d.session?.sessionKey ?? d.race.sessionKey) === activeSessionKey) {
              setF1Detail(d);
            }
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
        if ((payload?.session?.sessionKey ?? payload?.race?.sessionKey) === activeSessionKey) {
          setF1Detail(payload);
          setF1Races((prev) =>
            prev.map((r) =>
              r.meetingKey === payload.race.meetingKey ? { ...r, ...payload.race } : r,
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
    selectedMlbTeamId,
    mlbSlateDate,
    f1Year,
    setKeyRefreshing,
  ]);

  const mlbSidebarRefreshing = isRefreshing(
    "mlb.teams", "mlb.seasons", mlbScheduleKey, mlbStandingsKey,
    mlbDailyScheduleKey, mlbRosterKey,
  );
  const f1SidebarRefreshing = isRefreshing("f1.years", f1RacesKey, f1StandingsKey);
  const mlbScheduleRefreshing = isRefreshing(mlbScheduleKey);
  const mlbStandingsRefreshing = isRefreshing(mlbStandingsKey);
  const mlbDailyScheduleRefreshing = isRefreshing(mlbDailyScheduleKey);
  const mlbRosterRefreshing = isRefreshing(mlbRosterKey);
  const f1ListRefreshing = isRefreshing(f1RacesKey);
  const mlbDetailRefreshing = activePk != null && isRefreshing(`mlb.game.v2.${activePk}`);
  const f1DetailRefreshing =
    activeSessionKey != null && isRefreshing(`f1.race.${activeSessionKey}`);

  return (
    <div className="layout sports-layout">
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
                  if (sport.available) {
                    setActiveSport(sport.id);
                    if (sport.id !== "mlb" && mlbTeamRouteFromHash(window.location.hash)) {
                      window.location.hash = "";
                    }
                  }
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

                  <button
                    type="button"
                    className={`nav-item nav-item-nested sports-standings-entry ${
                      activeSport === "mlb" && mlbView === "standings" ? "active" : ""
                    }`}
                    onClick={selectMlbStandings}
                  >
                    <span className="sports-team-row"><span aria-hidden="true">▤</span> Standings &amp; today</span>
                  </button>

                  <div className="sports-team-group">
                    <div className="nav-item sports-team-heading">
                      <span>Teams</span>
                    </div>
                    {followedTeams.map((team) => (
                      <button
                        key={team.id}
                        type="button"
                        className={`nav-item nav-item-nested ${
                          activeSport === "mlb" && mlbView === "team" && selectedMlbTeamId === team.id ? "active" : ""
                        }`}
                        onClick={() => selectMlbTeam(team.id)}
                      >
                        <span className="sports-team-row">
                          {team.logoUrl ? <img src={team.logoUrl} alt="" className="sports-logo" /> : null}
                          {team.abbreviation || team.shortName || team.name}
                        </span>
                      </button>
                    ))}
                    {temporaryMlbTeam ? (
                      <div
                        className={`sports-temporary-team ${
                          activeSport === "mlb" && mlbView === "team" && selectedMlbTeamId === temporaryMlbTeam.id
                            ? "active"
                            : ""
                        }`}
                      >
                        <button
                          type="button"
                          className="nav-item nav-item-nested sports-temporary-team-link"
                          onClick={() => selectMlbTeam(temporaryMlbTeam.id)}
                        >
                          <span className="sports-team-row">
                            {temporaryMlbTeam.logoUrl ? (
                              <img src={temporaryMlbTeam.logoUrl} alt="" className="sports-logo" />
                            ) : null}
                            {temporaryMlbTeam.abbreviation ||
                              temporaryMlbTeam.shortName ||
                              temporaryMlbTeam.name}
                          </span>
                        </button>
                        <button
                          type="button"
                          className="sports-team-pin"
                          aria-label={`Keep ${temporaryMlbTeam.name} in the sidebar`}
                          title="Keep in sidebar"
                          disabled={pinningMlbTeamId === temporaryMlbTeam.id}
                          onClick={() => void pinMlbTeam(temporaryMlbTeam.id)}
                        >
                          {pinningMlbTeamId === temporaryMlbTeam.id ? "…" : "+"}
                        </button>
                      </div>
                    ) : null}
                  </div>
                  {followedTeams.length === 0 && !temporaryMlbTeam && (
                    <div className="empty" style={{ height: "auto", padding: 12 }}>
                      Follow MLB teams in Settings → Sports
                    </div>
                  )}
                </>
              )}

              {open && sport.available && sport.id === "f1" && (
                <>
                  <div className="section-label sports-sublabel">Year</div>
                  <select
                    className="sports-season-select"
                    aria-label="Formula 1 year"
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
                  <button
                    type="button"
                    className={`nav-item nav-item-nested sports-standings-entry ${
                      activeSport === "f1" ? "active" : ""
                    }`}
                    onClick={selectF1Standings}
                  >
                    <span className="sports-team-row"><span aria-hidden="true">▤</span> Standings</span>
                  </button>
                </>
              )}
            </div>
          );
        })}
      </aside>

      {activeSport === "mlb" ? (
        mlbView === "standings" ? (
          <BaseballStandingsDashboard
            season={season}
            standings={mlbStandings}
            games={dailyGames}
            gameDate={mlbSlateDate}
            todayDate={todayKey}
            followedTeamIds={followed}
            standingsLoading={mlbStandingsRefreshing}
            gamesLoading={mlbDailyScheduleRefreshing}
            error={error}
            onSelectTeam={selectMlbTeam}
            onSelectGame={selectMlbGame}
            onPreviousDay={() => setMlbSlateDate((date) => shiftLocalDateKey(date, -1))}
            onNextDay={() => setMlbSlateDate((date) => shiftLocalDateKey(date, 1))}
            onToday={() => setMlbSlateDate(localDateKey())}
          />
        ) : selectedMlbTeam ? (
          <BaseballTeamDashboard
            team={selectedMlbTeam}
            season={season}
            games={games}
            activeGameId={activePk}
            detail={detail}
            roster={roster}
            standings={mlbStandings}
            scheduleLoading={mlbScheduleRefreshing}
            detailLoading={mlbDetailRefreshing}
            rosterLoading={mlbRosterRefreshing}
            error={error}
            onSelectGame={setActivePk}
            onSelectTeam={selectMlbTeam}
          />
        ) : (
          <main className="mlb-dashboard pane">
            <div className="empty">
              <h2>Choose a baseball team</h2>
              <p>Follow a team in Settings, then select it in the sidebar to open its dashboard.</p>
              <button type="button" className="btn" onClick={() => onOpenSettingsSports?.()}>
                Manage baseball teams
              </button>
            </div>
          </main>
        )
      ) : activeSport === "f1" ? (
        <F1Dashboard
          year={f1Year}
          races={f1Races}
          activeMeetingKey={activeMeetingKey}
          activeSessionKey={activeSessionKey}
          activeSessionKind={f1SessionKind}
          detail={f1Detail}
          standings={f1Standings}
          availableKinds={availableKindPills}
          kindSessions={kindSessions}
          racesLoading={f1ListRefreshing}
          standingsLoading={isRefreshing(f1StandingsKey)}
          detailLoading={f1DetailRefreshing}
          error={error}
          onSelectRace={(race) => {
            setActiveMeetingKey(race.meetingKey);
            setF1SessionKind(defaultKind(race.sessions ?? []));
            setActiveSessionKey(race.sessionKey);
          }}
          onSelectKind={setF1SessionKind}
          onSelectSession={setActiveSessionKey}
        />
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
