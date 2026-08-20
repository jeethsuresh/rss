import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  DotaEvent,
  DotaEventTier,
  DotaGame,
  DotaMatch,
  DotaMatchDetail,
  DotaPinnedEvent,
  DotaProvidersStatus,
  DotaSeason,
  DotaTeam,
  ReaderBackend,
} from "@rss-reader/shared";
import { SportsLoadingPane, SportsSpinner } from "../components/SportsSpinner";
import { SPORTS_REGISTRY, type SportId } from "../lib/sportsRegistry";

type Props = {
  backend: ReaderBackend;
  activeSport: SportId;
  onSelectSport: (id: SportId) => void;
};

type Nav =
  | { type: "browse" }
  | { type: "event"; eventId: string }
  | { type: "team"; teamId: number }
  | { type: "match"; matchId: number; back: Nav };

const TIER_ORDER: DotaEventTier[] = [
  "premier",
  "professional",
  "semi-pro",
  "amateur",
  "unknown",
];

function tierLabel(t: DotaEventTier): string {
  switch (t) {
    case "premier":
      return "Premier";
    case "professional":
      return "Professional";
    case "semi-pro":
      return "Semi-Pro";
    case "amateur":
      return "Amateur";
    case "unknown":
      return "Unknown";
    default: {
      const _exhaustive: never = t;
      return _exhaustive;
    }
  }
}

function formatDuration(sec?: number): string {
  if (sec == null || sec <= 0) return "—";
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function scoreLine(m: DotaMatch): string {
  const a = m.scoreA ?? "–";
  const b = m.scoreB ?? "–";
  return `${m.teamA.name} ${a} — ${b} ${m.teamB.name}`;
}

export function DotaSportsPanel({ backend, activeSport, onSelectSport }: Props) {
  const [status, setStatus] = useState<DotaProvidersStatus | null>(null);
  const [years, setYears] = useState<DotaSeason[]>([]);
  const [year, setYear] = useState<number | null>(null);
  const [events, setEvents] = useState<DotaEvent[]>([]);
  const [pins, setPins] = useState<DotaPinnedEvent[]>([]);
  const [followed, setFollowed] = useState<number[]>([]);
  const [followedTeams, setFollowedTeams] = useState<DotaTeam[]>([]);
  const [nav, setNav] = useState<Nav>({ type: "browse" });
  const [matches, setMatches] = useState<DotaMatch[]>([]);
  const [matchDetail, setMatchDetail] = useState<DotaMatchDetail | null>(null);
  const [gameDetail, setGameDetail] = useState<DotaGame | null>(null);
  const [teamSearch, setTeamSearch] = useState("");
  const [searchHits, setSearchHits] = useState<DotaTeam[]>([]);
  const [listBusy, setListBusy] = useState(false);
  const [detailBusy, setDetailBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeEvent, setActiveEvent] = useState<DotaEvent | null>(null);

  const pinSet = useMemo(() => new Set(pins.map((p) => `${p.eventType}:${p.eventId}`)), [pins]);
  const followedSet = useMemo(() => new Set(followed), [followed]);

  const eventsByTier = useMemo(() => {
    const map = new Map<DotaEventTier, DotaEvent[]>();
    for (const t of TIER_ORDER) map.set(t, []);
    for (const ev of events) {
      const list = map.get(ev.tier) ?? map.get("unknown")!;
      list.push(ev);
    }
    return TIER_ORDER.map((t) => ({ tier: t, events: map.get(t) ?? [] })).filter(
      (g) => g.events.length > 0,
    );
  }, [events]);

  const pinnedEvents = useMemo(() => {
    const byId = new Map(events.map((e) => [e.id, e]));
    return pins
      .map((p) => byId.get(p.eventId))
      .filter((e): e is DotaEvent => !!e && e.year === year);
  }, [pins, events, year]);

  useEffect(() => {
    void (async () => {
      try {
        const [st, ys, fl, pn] = await Promise.all([
          backend.sports.dotaStatus(),
          backend.sports.dotaYears(),
          backend.sports.dotaFollowedGet(),
          backend.sports.dotaPinnedGet(),
        ]);
        setStatus(st);
        setYears(ys ?? []);
        setYear(ys?.[0]?.year ?? new Date().getFullYear());
        setFollowed(fl ?? []);
        setPins(pn ?? []);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load Dota");
      }
    })();
  }, [backend]);

  useEffect(() => {
    if (year == null) return;
    setListBusy(true);
    setError(null);
    setEvents([]);
    setMatches([]);
    setMatchDetail(null);
    setGameDetail(null);
    void backend.sports
      .dotaEvents({ year })
      .then((list) => setEvents(list ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load events"))
      .finally(() => setListBusy(false));
  }, [backend, year]);

  useEffect(() => {
    if (followed.length === 0) {
      setFollowedTeams([]);
      return;
    }
    let cancelled = false;
    void Promise.all(
      followed.map((id) =>
        backend.sports.dotaTeamGet(id).catch(() => ({ id, name: `Team ${id}` }) as DotaTeam),
      ),
    ).then((teams) => {
      if (!cancelled) setFollowedTeams(teams);
    });
    return () => {
      cancelled = true;
    };
  }, [backend, followed]);

  const openEvent = useCallback(
    async (eventId: string) => {
      setNav({ type: "event", eventId });
      setGameDetail(null);
      setMatchDetail(null);
      setMatches([]);
      setListBusy(true);
      setError(null);
      try {
        const ev = events.find((e) => e.id === eventId) ?? null;
        setActiveEvent(ev);
        const list = await backend.sports.dotaEventMatches(eventId);
        setMatches(list ?? []);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load matches");
      } finally {
        setListBusy(false);
      }
    },
    [backend, events],
  );

  const openTeam = useCallback(
    async (teamId: number) => {
      setNav({ type: "team", teamId });
      setGameDetail(null);
      setMatchDetail(null);
      setMatches([]);
      setListBusy(true);
      setError(null);
      try {
        const list = await backend.sports.dotaTeamMatches({
          teamId,
          year: year ?? undefined,
        });
        setMatches(list ?? []);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load team matches");
      } finally {
        setListBusy(false);
      }
    },
    [backend, year],
  );

  const openMatch = useCallback(
    async (matchId: number, back: Nav) => {
      setNav({ type: "match", matchId, back });
      setGameDetail(null);
      setMatchDetail(null);
      setListBusy(true);
      setDetailBusy(true);
      setError(null);
      try {
        const detail = await backend.sports.dotaMatchWatch(matchId);
        setMatchDetail(detail);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load match");
      } finally {
        setListBusy(false);
        setDetailBusy(false);
      }
    },
    [backend],
  );

  useEffect(() => {
    if (nav.type !== "match") {
      if (matchDetail?.match.id) {
        void backend.sports.dotaMatchUnwatch(matchDetail.match.id).catch(() => undefined);
      }
      return;
    }
    const matchId = nav.matchId;
    return backend.onEvent((ev) => {
      if (ev.event !== "sports.dota.match.updated") return;
      const payload = ev.payload as { matchId?: number; detail?: DotaMatchDetail };
      if (payload.matchId === matchId && payload.detail) {
        setMatchDetail(payload.detail);
      }
    });
  }, [backend, nav, matchDetail?.match.id]);

  const openGame = useCallback(
    async (g: DotaGame) => {
      setGameDetail(null);
      setBusy(true);
      setError(null);
      try {
        const detail = await backend.sports.dotaGameGet({
          matchId: g.matchId,
          gameIndex: g.gameIndex,
          stratzMatchId: g.stratzMatchId,
        });
        setGameDetail(detail);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load game");
      } finally {
        setBusy(false);
      }
    },
    [backend],
  );

  const togglePin = async (ev: DotaEvent) => {
    const next = await backend.sports.dotaPinnedToggle(ev.id, ev.type);
    setPins(next ?? []);
  };

  const toggleFollow = async (teamId: number) => {
    const next = await backend.sports.dotaFollowedToggle(teamId);
    setFollowed(next ?? []);
  };

  useEffect(() => {
    const q = teamSearch.trim();
    if (q.length < 2) {
      setSearchHits([]);
      return;
    }
    const t = setTimeout(() => {
      void backend.sports
        .dotaTeamSearch(q)
        .then((hits) => setSearchHits(hits ?? []))
        .catch(() => setSearchHits([]));
    }, 300);
    return () => clearTimeout(t);
  }, [backend, teamSearch]);

  const bucketed = useMemo(() => {
    const upcoming: DotaMatch[] = [];
    const live: DotaMatch[] = [];
    const completed: DotaMatch[] = [];
    for (const m of matches) {
      switch (m.status) {
        case "live":
          live.push(m);
          break;
        case "completed":
          completed.push(m);
          break;
        case "upcoming":
          upcoming.push(m);
          break;
        default: {
          const _exhaustive: never = m.status;
          void _exhaustive;
        }
      }
    }
    return { upcoming, live, completed };
  }, [matches]);

  if (status && !status.pandaScoreConfigured) {
    return (
      <div className="layout">
        <aside className="pane sidebar">
          {SPORTS_REGISTRY.map((sport) => (
            <button
              key={sport.id}
              type="button"
              className={`nav-item ${activeSport === sport.id ? "active" : ""}`}
              onClick={() => onSelectSport(sport.id)}
            >
              {sport.label}
            </button>
          ))}
        </aside>
        <section className="pane article-list">
          <div className="empty">
            <h2>Dota 2</h2>
            <p>
              Set <code>PANDASCORE_API_TOKEN</code> in the environment and restart the app. Optional:{" "}
              <code>STRATZ_API_TOKEN</code> for game details.
            </p>
          </div>
        </section>
        <section className="pane reader" />
      </div>
    );
  }

  return (
    <div className="layout">
      <aside className="pane sidebar">
        {SPORTS_REGISTRY.map((sport) => (
          <button
            key={sport.id}
            type="button"
            className={`nav-item ${activeSport === sport.id ? "active" : ""}`}
            onClick={() => onSelectSport(sport.id)}
          >
            {sport.label}
          </button>
        ))}
        <div className="section-label">Dota 2</div>
        <div className="section-label">Year</div>
        {years.map((y) => (
          <button
            key={y.year}
            type="button"
            className={`nav-item ${year === y.year ? "active" : ""}`}
            onClick={() => {
              setYear(y.year);
              setNav({ type: "browse" });
              setMatchDetail(null);
              setGameDetail(null);
            }}
          >
            {y.year}
          </button>
        ))}

        <div className="section-label">Following</div>
        {followedTeams.length === 0 && (
          <div className="empty" style={{ height: "auto", padding: 8 }}>
            Follow teams via search
          </div>
        )}
        {followedTeams.map((t) => (
          <button
            key={t.id}
            type="button"
            className={`nav-item nav-item-nested ${
              nav.type === "team" && nav.teamId === t.id ? "active" : ""
            }`}
            onClick={() => void openTeam(t.id)}
          >
            ★ {t.name}
          </button>
        ))}

        <div className="section-label">Pinned</div>
        {pinnedEvents.length === 0 && (
          <div className="empty" style={{ height: "auto", padding: 8 }}>
            Pin events from Browse
          </div>
        )}
        {pinnedEvents.map((ev) => (
          <button
            key={ev.id}
            type="button"
            className={`nav-item nav-item-nested ${
              nav.type === "event" && nav.eventId === ev.id ? "active" : ""
            }`}
            onClick={() => void openEvent(ev.id)}
          >
            ★ {ev.name}
          </button>
        ))}

        <div className="section-label">Browse</div>
        <button
          type="button"
          className={`nav-item ${nav.type === "browse" ? "active" : ""}`}
          onClick={() => {
            setNav({ type: "browse" });
            setMatchDetail(null);
            setGameDetail(null);
          }}
        >
          Tournaments &amp; Leagues
        </button>
        <div style={{ padding: "8px 10px" }}>
          <input
            placeholder="Search teams…"
            value={teamSearch}
            onChange={(e) => setTeamSearch(e.target.value)}
            style={{ width: "100%" }}
          />
          {searchHits.map((t) => (
            <div key={t.id} className="nav-item nav-item-nested" style={{ display: "flex", gap: 6 }}>
              <button type="button" style={{ flex: 1, textAlign: "left" }} onClick={() => void openTeam(t.id)}>
                {t.name}
              </button>
              <button type="button" className="btn" onClick={() => void toggleFollow(t.id)}>
                {followedSet.has(t.id) ? "Unfollow" : "Follow"}
              </button>
            </div>
          ))}
        </div>
      </aside>

      <section className="pane article-list">
        {error && <p className="error">{error}</p>}
        {busy ? (
          <SportsLoadingPane label="Loading…" />
        ) : (
          <>
        {nav.type === "browse" && (
          <>
            <h2 style={{ padding: "8px 12px", margin: 0 }}>{year} · Events</h2>
            {eventsByTier.map((group) => (
              <div key={group.tier}>
                <div className="section-label">{tierLabel(group.tier)}</div>
                {group.events.map((ev) => {
                  const pinned = pinSet.has(`${ev.type}:${ev.id}`);
                  return (
                    <button
                      key={ev.id}
                      type="button"
                      className="article-row"
                      onClick={() => void openEvent(ev.id)}
                    >
                      <div className="article-meta">
                        <span>{tierLabel(ev.tier)}</span>
                        <span>{ev.status}</span>
                        {ev.organizer ? <span>{ev.organizer}</span> : null}
                      </div>
                      <h3 className="article-title">{ev.name}</h3>
                      <div className="modal-actions" style={{ justifyContent: "flex-start" }}>
                        <button
                          type="button"
                          className="btn"
                          onClick={(e) => {
                            e.stopPropagation();
                            void togglePin(ev);
                          }}
                        >
                          {pinned ? "Unpin" : "Pin"}
                        </button>
                      </div>
                    </button>
                  );
                })}
              </div>
            ))}
            {events.length === 0 && (
              <div className="empty">
                <h2>No events</h2>
                <p>No PandaScore series found for {year}.</p>
              </div>
            )}
          </>
        )}

        {(nav.type === "event" || nav.type === "team") && !matchDetail && (
          <>
            <div style={{ padding: "8px 12px", display: "flex", gap: 8, alignItems: "center" }}>
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setNav({ type: "browse" });
                  setMatches([]);
                }}
              >
                ← Back
              </button>
              <h2 style={{ margin: 0 }}>
                {nav.type === "event"
                  ? activeEvent?.name ?? "Event"
                  : followedTeams.find((t) => t.id === nav.teamId)?.name ?? `Team ${nav.teamId}`}
              </h2>
              {nav.type === "event" && activeEvent ? (
                <span className="count">{tierLabel(activeEvent.tier)}</span>
              ) : null}
            </div>
            {(
              [
                ["Live", bucketed.live],
                ["Upcoming", bucketed.upcoming],
                ["Completed", bucketed.completed],
              ] as const
            ).map(([label, list]) =>
              list.length === 0 ? null : (
                <div key={label}>
                  <div className="section-label">{label}</div>
                  {list.map((m) => (
                    <button
                      key={m.id}
                      type="button"
                      className="article-row"
                      onClick={() => void openMatch(m.id, nav)}
                    >
                      <div className="article-meta">
                        <span>{m.status}</span>
                        {m.stage ? <span>{m.stage}</span> : null}
                        {m.eventName ? <span>{m.eventName}</span> : null}
                      </div>
                      <h3 className="article-title">{scoreLine(m)}</h3>
                      <div className="modal-actions" style={{ justifyContent: "flex-start", gap: 6 }}>
                        <button
                          type="button"
                          className="btn"
                          onClick={(e) => {
                            e.stopPropagation();
                            void toggleFollow(m.teamA.id);
                          }}
                        >
                          {followedSet.has(m.teamA.id) ? "★" : "☆"} {m.teamA.shortName || m.teamA.name}
                        </button>
                        <button
                          type="button"
                          className="btn"
                          onClick={(e) => {
                            e.stopPropagation();
                            void toggleFollow(m.teamB.id);
                          }}
                        >
                          {followedSet.has(m.teamB.id) ? "★" : "☆"} {m.teamB.shortName || m.teamB.name}
                        </button>
                      </div>
                    </button>
                  ))}
                </div>
              ),
            )}
          </>
        )}

        {nav.type === "match" && matchDetail && (
          <>
            <div style={{ padding: "8px 12px", display: "flex", gap: 8, alignItems: "center" }}>
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setNav(nav.back);
                  setMatchDetail(null);
                  setGameDetail(null);
                }}
              >
                ← Back
              </button>
              <h2 style={{ margin: 0 }}>{scoreLine(matchDetail.match)}</h2>
              {matchDetail.live ? <SportsSpinner label="Live" /> : null}
            </div>
            <p style={{ padding: "0 12px", color: "var(--muted)" }}>
              {matchDetail.match.eventName}
              {matchDetail.match.stage ? ` · ${matchDetail.match.stage}` : ""}
              {matchDetail.match.scheduledAt
                ? ` · ${new Date(matchDetail.match.scheduledAt).toLocaleString()}`
                : ""}
            </p>
            <div className="section-label">Games</div>
            {(matchDetail.games ?? []).length === 0 && (
              <div className="empty" style={{ height: "auto", padding: 12 }}>
                No individual games yet
              </div>
            )}
            {(matchDetail.games ?? []).map((g) => (
              <button
                key={g.id}
                type="button"
                className={`article-row ${gameDetail?.id === g.id ? "active" : ""}`}
                onClick={() => void openGame(g)}
              >
                <div className="article-meta">
                  <span>Game {g.gameIndex}</span>
                  <span>{formatDuration(g.durationSeconds)}</span>
                </div>
                <h3 className="article-title">
                  {g.winner ? `${g.winner} win` : "Result pending"}
                </h3>
              </button>
            ))}
          </>
        )}
          </>
        )}
      </section>

      <section className="pane reader">
        {busy && !gameDetail && nav.type === "match" ? (
          <SportsLoadingPane label="Loading…" />
        ) : !gameDetail && !matchDetail && nav.type === "browse" ? (
          <div className="empty">
            <h2>Dota 2</h2>
            <p>Pick a tournament or followed team. STRATZ details load only when you open a game.</p>
            {status && !status.stratzConfigured ? (
              <p className="muted">STRATZ token not set — series scores still work from PandaScore.</p>
            ) : null}
          </div>
        ) : busy && !gameDetail ? (
          <SportsLoadingPane label="Loading…" />
        ) : gameDetail ? (
          <article className="reader-body" style={{ padding: 16 }}>
            <h1>Game {gameDetail.gameIndex}</h1>
            {!gameDetail.detailAvailable && (
              <p className="error">
                {gameDetail.detailError || "Detailed statistics unavailable"}
              </p>
            )}
            <p>
              Duration {formatDuration(gameDetail.durationSeconds)}
              {gameDetail.winner ? ` · ${gameDetail.winner} win` : ""}
              {gameDetail.radiantScore != null && gameDetail.direScore != null
                ? ` · Kills ${gameDetail.radiantScore}–${gameDetail.direScore}`
                : ""}
            </p>
            {gameDetail.heroes && gameDetail.heroes.length > 0 && (
              <>
                <h3>Heroes</h3>
                <ul>
                  {gameDetail.heroes.map((h, i) => (
                    <li key={`${h.heroId}-${i}`}>
                      {h.team}: {h.heroName}
                    </li>
                  ))}
                </ul>
              </>
            )}
            {gameDetail.bans && gameDetail.bans.length > 0 && (
              <>
                <h3>Bans</h3>
                <ul>
                  {gameDetail.bans.map((h, i) => (
                    <li key={`${h.heroId}-b-${i}`}>
                      {h.team}: {h.heroName}
                    </li>
                  ))}
                </ul>
              </>
            )}
            {gameDetail.players && gameDetail.players.length > 0 && (
              <>
                <h3>Players</h3>
                <table style={{ width: "100%", fontSize: "0.85rem" }}>
                  <thead>
                    <tr>
                      <th align="left">Player</th>
                      <th align="left">Hero</th>
                      <th>K</th>
                      <th>D</th>
                      <th>A</th>
                      <th>GPM</th>
                      <th>XPM</th>
                      <th>NW</th>
                    </tr>
                  </thead>
                  <tbody>
                    {gameDetail.players.map((p) => (
                      <tr key={p.playerId}>
                        <td>{p.name}</td>
                        <td>{p.heroName}</td>
                        <td align="center">{p.kills}</td>
                        <td align="center">{p.deaths}</td>
                        <td align="center">{p.assists}</td>
                        <td align="center">{p.gpm}</td>
                        <td align="center">{p.xpm}</td>
                        <td align="center">{p.netWorth}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </article>
        ) : matchDetail ? (
          <div className="empty">
            <h2>Series</h2>
            <p>Select a game for STRATZ stats when a Steam match id is known.</p>
          </div>
        ) : (
          <div className="empty" />
        )}
      </section>
    </div>
  );
}
