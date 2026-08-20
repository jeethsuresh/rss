import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  F1Race,
  F1RaceDetail,
  F1RaceStatus,
  F1Season,
  MlbGame,
  MlbGameDetail,
  MlbGameStatus,
  MlbSeason,
  MlbTeam,
  ReaderBackend,
} from "@rss-reader/shared";
import { SPORTS_REGISTRY, type SportId } from "../lib/sportsRegistry";

type Props = {
  backend: ReaderBackend;
  onOpenSettingsSports?: () => void;
};

type GameBucket = "completed" | "in_progress" | "scheduled";

type MlbSelection =
  | { type: "all"; bucket: GameBucket }
  | { type: "team"; id: number; bucket: GameBucket };

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

  // --- F1 ---
  const [f1Years, setF1Years] = useState<F1Season[]>([]);
  const [f1Year, setF1Year] = useState<number | null>(null);
  const [f1BucketSel, setF1BucketSel] = useState<GameBucket>("completed");
  const [f1Races, setF1Races] = useState<F1Race[]>([]);
  const [activeSessionKey, setActiveSessionKey] = useState<number | null>(null);
  const [f1Detail, setF1Detail] = useState<F1RaceDetail | null>(null);
  const [significantOnly, setSignificantOnly] = useState(true);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expandedSports, setExpandedSports] = useState<ReadonlySet<SportId>>(
    () => new Set<SportId>(["mlb"]),
  );

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

  const selectF1Bucket = (bucket: GameBucket) => {
    setActiveSport("f1");
    setF1BucketSel(bucket);
  };

  const followedTeams = useMemo(
    () => teams.filter((t) => followed.includes(t.id)).sort((a, b) => a.name.localeCompare(b.name)),
    [teams, followed],
  );

  const reloadMlbMeta = useCallback(async () => {
    const [t, f, s] = await Promise.all([
      backend.sports.teams(),
      backend.sports.followedGet(),
      backend.sports.seasons(),
    ]);
    setTeams(t ?? []);
    setFollowed(f ?? []);
    setSeasons(s ?? []);
    setSeason((prev) => prev ?? s?.[0]?.seasonId ?? new Date().getFullYear());
  }, [backend]);

  const reloadF1Meta = useCallback(async () => {
    const years = await backend.sports.f1Years();
    setF1Years(years ?? []);
    setF1Year((prev) => prev ?? years?.[0]?.year ?? new Date().getFullYear());
  }, [backend]);

  const scheduleTeamId = selection.type === "team" ? selection.id : undefined;

  const reloadMlbSchedule = useCallback(async () => {
    if (season == null || activeSport !== "mlb") return;
    setBusy(true);
    setError(null);
    try {
      const list = await backend.sports.schedule({ teamId: scheduleTeamId, season });
      setGames(list ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load schedule");
      setGames([]);
    } finally {
      setBusy(false);
    }
  }, [backend, season, scheduleTeamId, activeSport]);

  const reloadF1Races = useCallback(async () => {
    if (f1Year == null || activeSport !== "f1") return;
    setBusy(true);
    setError(null);
    try {
      const list = await backend.sports.f1Races({ year: f1Year });
      setF1Races(list ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load races");
      setF1Races([]);
    } finally {
      setBusy(false);
    }
  }, [backend, f1Year, activeSport]);

  const visibleGames = useMemo(() => {
    return games.filter((g) => gameBucket(g.status) === selection.bucket);
  }, [games, selection.bucket]);

  const visibleRaces = useMemo(() => {
    return f1Races.filter((r) => f1Bucket(r.status) === f1BucketSel);
  }, [f1Races, f1BucketSel]);

  useEffect(() => {
    setActivePk((pk) => {
      if (pk && visibleGames.some((g) => g.id === pk)) return pk;
      return visibleGames[0]?.id ?? null;
    });
  }, [visibleGames]);

  useEffect(() => {
    setActiveSessionKey((key) => {
      if (key && visibleRaces.some((r) => r.sessionKey === key)) return key;
      return visibleRaces[0]?.sessionKey ?? null;
    });
  }, [visibleRaces]);

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
    void reloadF1Races();
  }, [reloadF1Races]);

  useEffect(() => {
    if (activeSport !== "mlb" || !activePk) {
      if (activeSport !== "mlb") setDetail(null);
      return;
    }
    let cancelled = false;
    void backend.sports
      .gameWatch(activePk)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load game");
      });
    return () => {
      cancelled = true;
      void backend.sports.gameUnwatch(activePk);
    };
  }, [activePk, backend, activeSport]);

  useEffect(() => {
    if (activeSport !== "f1" || !activeSessionKey) {
      if (activeSport !== "f1") setF1Detail(null);
      return;
    }
    let cancelled = false;
    void backend.sports
      .f1RaceWatch(activeSessionKey)
      .then((d) => {
        if (!cancelled) setF1Detail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load race");
      });
    return () => {
      cancelled = true;
      void backend.sports.f1RaceUnwatch(activeSessionKey);
    };
  }, [activeSessionKey, backend, activeSport]);

  useEffect(() => {
    setScoringOnly(false);
  }, [activePk]);

  useEffect(() => {
    setSignificantOnly(true);
  }, [activeSessionKey]);

  useEffect(() => {
    return backend.onEvent((ev) => {
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
  }, [backend, activePk, activeSessionKey]);

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
                      <span>Races</span>
                    </div>
                    {BUCKETS.map((b) => (
                      <button
                        key={`f1-${b.id}`}
                        type="button"
                        className={`nav-item nav-item-nested ${
                          activeSport === "f1" && f1BucketSel === b.id ? "active" : ""
                        }`}
                        onClick={() => selectF1Bucket(b.id)}
                      >
                        <span>{b.label}</span>
                      </button>
                    ))}
                  </div>
                </>
              )}

              {open && sport.available && sport.id === "nfl" ? (
                <p className="sports-coming-soon muted">
                  {sport.label} is marked available but has no UI panel yet.
                </p>
              ) : null}
            </div>
          );
        })}
      </aside>

      {activeSport === "mlb" ? (
        <>
          <section className="pane article-list">
            {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
            {busy && visibleGames.length === 0 ? (
              <div className="empty">
                <p>Loading schedule…</p>
              </div>
            ) : visibleGames.length === 0 ? (
              <div className="empty">
                <h2>No games</h2>
                <p>
                  {`No ${BUCKETS.find((b) => b.id === selection.bucket)?.label.toLowerCase() ?? ""} games${
                    selection.type === "team" ? " for this team" : ""
                  } in ${season}.`}
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
              <div className="empty">
                <h2>Sports</h2>
                <p>Select a game to see the score, linescore, and plays.</p>
              </div>
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
                  <h2 className="sports-plays-heading">Plays</h2>
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
                </div>
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
                          {p.isScoringPlay ? <span className="sports-scoring-badge">Scoring</span> : null}
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
              </article>
            )}
          </section>
        </>
      ) : activeSport === "f1" ? (
        <>
          <section className="pane article-list">
            {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
            {busy && visibleRaces.length === 0 ? (
              <div className="empty">
                <p>Loading races…</p>
              </div>
            ) : visibleRaces.length === 0 ? (
              <div className="empty">
                <h2>No races</h2>
                <p>
                  {`No ${BUCKETS.find((b) => b.id === f1BucketSel)?.label.toLowerCase() ?? ""} races in ${f1Year}.`}
                </p>
              </div>
            ) : (
              visibleRaces.map((r) => (
                <button
                  key={r.sessionKey}
                  type="button"
                  className={`article-row ${r.sessionKey === activeSessionKey ? "active" : ""}`}
                  onClick={() => setActiveSessionKey(r.sessionKey)}
                >
                  <div className="article-meta">
                    <span>{f1StatusLabel(r.status)}</span>
                    <span>{r.countryCode || r.countryName}</span>
                  </div>
                  <h3 className="article-title">{r.name}</h3>
                  <p className="article-summary">
                    {r.circuitShortName || r.location}
                    {" · "}
                    {r.dateStart ? new Date(r.dateStart).toLocaleString() : ""}
                  </p>
                </button>
              ))
            )}
          </section>

          <section className="pane reader-pane">
            {!f1Detail ? (
              <div className="empty">
                <h2>F1</h2>
                <p>Select a race to see classification and race-control events.</p>
              </div>
            ) : (
              <article className="reader sports-reader">
                <div className="reader-kicker">
                  {f1StatusLabel(f1Detail.race.status)}
                  {f1Detail.race.countryName ? ` · ${f1Detail.race.countryName}` : ""}
                </div>
                <h1 className="sports-scoreline" style={{ fontSize: "1.35rem" }}>
                  {f1Detail.race.name}
                </h1>
                <p className="muted" style={{ marginTop: -8, marginBottom: 16 }}>
                  {f1Detail.race.circuitShortName || f1Detail.race.location}
                  {f1Detail.race.dateStart
                    ? ` · ${new Date(f1Detail.race.dateStart).toLocaleString()}`
                    : ""}
                </p>

                <h2 className="sports-plays-heading">Classification</h2>
                {f1Detail.results.length === 0 ? (
                  <p className="muted">No results yet.</p>
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
                )}

                <div className="sports-plays-header">
                  <h2 className="sports-plays-heading">Race control</h2>
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
                </div>
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
                          <span>{e.category}{e.flag ? ` · ${e.flag}` : ""}</span>
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
              </article>
            )}
          </section>
        </>
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
