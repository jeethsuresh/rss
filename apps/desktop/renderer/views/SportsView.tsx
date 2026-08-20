import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  MlbGame,
  MlbGameDetail,
  MlbGameStatus,
  MlbSeason,
  MlbTeam,
  ReaderBackend,
} from "@rss-reader/shared";

type Props = {
  backend: ReaderBackend;
  onOpenSettingsSports?: () => void;
};

type GameBucket = "completed" | "in_progress" | "scheduled";

type Selection =
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

function scoreText(g: MlbGame): string {
  if (g.awayScore == null || g.homeScore == null) return "—";
  return `${g.awayScore}–${g.homeScore}`;
}

function matchup(g: MlbGame): string {
  const away = g.awayTeam.abbreviation || g.awayTeam.shortName || g.awayTeam.name;
  const home = g.homeTeam.abbreviation || g.homeTeam.shortName || g.homeTeam.name;
  return `${away} @ ${home}`;
}

export function SportsView({ backend, onOpenSettingsSports }: Props) {
  const [teams, setTeams] = useState<MlbTeam[]>([]);
  const [followed, setFollowed] = useState<number[]>([]);
  const [seasons, setSeasons] = useState<MlbSeason[]>([]);
  const [season, setSeason] = useState<number | null>(null);
  const [selection, setSelection] = useState<Selection>({ type: "all", bucket: "scheduled" });
  const [games, setGames] = useState<MlbGame[]>([]);
  const [activePk, setActivePk] = useState<number | null>(null);
  const [detail, setDetail] = useState<MlbGameDetail | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [scoringOnly, setScoringOnly] = useState(false);

  const followedTeams = useMemo(
    () => teams.filter((t) => followed.includes(t.id)).sort((a, b) => a.name.localeCompare(b.name)),
    [teams, followed],
  );

  const reloadMeta = useCallback(async () => {
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

  const scheduleTeamId = selection.type === "team" ? selection.id : undefined;

  const reloadSchedule = useCallback(async () => {
    if (season == null) return;
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
  }, [backend, season, scheduleTeamId]);

  const visibleGames = useMemo(() => {
    return games.filter((g) => gameBucket(g.status) === selection.bucket);
  }, [games, selection.bucket]);

  useEffect(() => {
    setActivePk((pk) => {
      if (pk && visibleGames.some((g) => g.id === pk)) return pk;
      return visibleGames[0]?.id ?? null;
    });
  }, [visibleGames]);

  useEffect(() => {
    void reloadMeta().catch((e) => setError(e instanceof Error ? e.message : "Failed to load sports"));
  }, [reloadMeta]);

  useEffect(() => {
    void reloadSchedule();
  }, [reloadSchedule]);

  useEffect(() => {
    if (!activePk) {
      setDetail(null);
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
  }, [activePk, backend]);

  useEffect(() => {
    setScoringOnly(false);
  }, [activePk]);

  useEffect(() => {
    return backend.onEvent((ev) => {
      if (ev.event !== "sports.game.updated") return;
      const payload = ev.payload as MlbGameDetail;
      if (payload?.game?.id === activePk) {
        setDetail(payload);
        setGames((prev) =>
          prev.map((g) => (g.id === payload.game.id ? { ...g, ...payload.game } : g)),
        );
      }
    });
  }, [backend, activePk]);

  const visiblePlays = useMemo(() => {
    if (!detail) return [];
    return scoringOnly ? detail.plays.filter((p) => p.isScoringPlay) : detail.plays;
  }, [detail, scoringOnly]);

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
        <div className="section-label">Season</div>
        <select
          className="sports-season-select"
          value={season ?? ""}
          onChange={(e) => setSeason(Number(e.target.value))}
        >
          {seasons.map((s) => (
            <option key={s.seasonId} value={s.seasonId}>
              {s.seasonId}
            </option>
          ))}
        </select>

        <div className="section-label">Teams</div>
        <div className="sports-team-group">
          <div className="nav-item sports-team-heading">
            <span>All followed</span>
          </div>
          {BUCKETS.map((b) => (
            <button
              key={`all-${b.id}`}
              type="button"
              className={`nav-item nav-item-nested ${
                selection.type === "all" && selection.bucket === b.id ? "active" : ""
              }`}
              onClick={() => setSelection({ type: "all", bucket: b.id })}
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
                  selection.type === "team" && selection.id === t.id && selection.bucket === b.id
                    ? "active"
                    : ""
                }`}
                onClick={() => setSelection({ type: "team", id: t.id, bucket: b.id })}
              >
                <span>{b.label}</span>
              </button>
            ))}
          </div>
        ))}
        {followedTeams.length === 0 && (
          <div className="empty" style={{ height: "auto", padding: 12 }}>
            Follow teams in Settings → Sports
          </div>
        )}
        <button type="button" className="nav-item" onClick={() => onOpenSettingsSports?.()}>
          Manage teams…
        </button>
      </aside>

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
    </div>
  );
}
