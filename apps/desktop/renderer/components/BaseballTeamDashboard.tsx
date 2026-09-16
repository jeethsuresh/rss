import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type {
  MlbGame,
  MlbGameDetail,
  MlbRoster,
  MlbRosterPlayer,
  MlbStandings,
  MlbTeam,
} from "@rss-reader/shared";
import {
  chronologicalGames,
  gameDateKey,
  localDateKey,
  pitcherChanges,
  pitcherStints,
  preferredGameId,
  teamGameResult,
  teamPlayoffOutlook,
} from "../lib/baseballDashboard";
import { mlbTeamHref } from "../lib/sportsDeepLinks";
import { SportsLoadingPane, SportsSpinner } from "./SportsSpinner";

type Props = {
  team: MlbTeam;
  season: number | null;
  games: MlbGame[];
  activeGameId: number | null;
  detail: MlbGameDetail | null;
  roster: MlbRoster | null;
  standings: MlbStandings | null;
  scheduleLoading: boolean;
  detailLoading: boolean;
  rosterLoading: boolean;
  error: string | null;
  onSelectGame: (gameId: number) => void;
  onSelectTeam: (teamId: number) => void;
};

function teamLabel(team: MlbTeam): string {
  return team.abbreviation || team.shortName || team.name;
}

function TeamLink({
  team,
  season,
  onSelectTeam,
  className,
  children,
}: {
  team: MlbTeam;
  season: number | null;
  onSelectTeam: (teamId: number) => void;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <a
      className={`mlb-team-link${className ? ` ${className}` : ""}`}
      href={mlbTeamHref(team.id, season)}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onSelectTeam(team.id);
      }}
    >
      {children ?? teamLabel(team)}
    </a>
  );
}

function statusLabel(game: MlbGame): string {
  switch (game.status) {
    case "live":
      return game.statusDetail || "Live";
    case "final":
      return "Final";
    case "scheduled":
      return "Scheduled";
    case "pre_game":
      return game.statusDetail || "Pre-game";
    case "postponed":
      return "Postponed";
    case "cancelled":
      return "Cancelled";
    case "unknown":
      return game.statusDetail || "Status pending";
    default: {
      const _exhaustive: never = game.status;
      return _exhaustive;
    }
  }
}

function formatGameDay(game: MlbGame): string {
  const date = new Date(`${gameDateKey(game)}T12:00:00`);
  if (Number.isNaN(date.getTime())) return gameDateKey(game);
  return date.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" });
}

function formatGameTime(game: MlbGame): string {
  const date = new Date(game.gameDate);
  if (Number.isNaN(date.getTime())) return "Time TBD";
  return date.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

function gameContext(game: MlbGame): string {
  if (game.currentInning) {
    return `${game.currentInningHalf === "top" ? "Top" : "Bot"} ${game.currentInning}`;
  }
  if (game.status === "scheduled" || game.status === "pre_game" || game.status === "unknown") {
    return formatGameTime(game);
  }
  return statusLabel(game);
}

function ScheduleRail({
  team,
  games,
  activeGameId,
  loading,
  onSelectGame,
  season,
  onSelectTeam,
  standings,
}: Pick<
  Props,
  "team" | "season" | "games" | "activeGameId" | "onSelectGame" | "onSelectTeam" | "standings"
> & { loading: boolean }) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const centeredGameRef = useRef<number | null>(null);
  const orderedGames = useMemo(() => chronologicalGames(games), [games]);
  const today = localDateKey();
  const todayGameId = useMemo(() => preferredGameId(orderedGames), [orderedGames]);
  const playoffOutlook = useMemo(
    () => teamPlayoffOutlook(standings, team.id),
    [standings, team.id],
  );

  useEffect(() => {
    const scroller = scrollerRef.current;
    const active = scroller?.querySelector<HTMLElement>(`[data-game-id="${activeGameId}"]`);
    if (!scroller || !active) return;
    const frame = requestAnimationFrame(() => {
      const scrollerRect = scroller.getBoundingClientRect();
      const activeRect = active.getBoundingClientRect();
      const left =
        scroller.scrollLeft +
        activeRect.left +
        activeRect.width / 2 -
        scrollerRect.left -
        scrollerRect.width / 2;
      scroller.scrollTo({
        left,
        behavior: centeredGameRef.current == null ? "auto" : "smooth",
      });
      centeredGameRef.current = activeGameId;
    });
    return () => cancelAnimationFrame(frame);
  }, [activeGameId, team.id, orderedGames.length]);

  const selectAdjacent = (direction: -1 | 1) => {
    const index = orderedGames.findIndex((game) => game.id === activeGameId);
    const next = orderedGames[index + direction];
    if (next) onSelectGame(next.id);
  };

  return (
    <section className="mlb-schedule-section" aria-label={`${team.name} schedule`}>
      <div className="mlb-section-heading">
        <div>
          <h1>
            {team.logoUrl ? <img src={team.logoUrl} alt="" className="mlb-dashboard-team-logo" /> : null}
            <TeamLink team={team} season={season} onSelectTeam={onSelectTeam}>
              {team.name}
            </TeamLink>
            {playoffOutlook ? (
              <span
                className={`mlb-playoff-outlook ${playoffOutlook.kind}`}
                aria-label={`${playoffOutlook.title}. ${playoffOutlook.detail}`}
                title={playoffOutlook.detail}
                tabIndex={0}
              >
                <strong>{playoffOutlook.title}</strong>
                <span className="mlb-playoff-outlook-detail" aria-hidden="true">
                  {playoffOutlook.detail}
                </span>
              </span>
            ) : null}
          </h1>
        </div>
        <div className="mlb-schedule-controls">
          {loading ? <SportsSpinner label="Updating schedule" /> : null}
          <button
            type="button"
            aria-label="Previous game"
            disabled={orderedGames.findIndex((game) => game.id === activeGameId) <= 0}
            onClick={() => selectAdjacent(-1)}
          >←</button>
          <button
            type="button"
            className="mlb-today-button"
            disabled={todayGameId == null}
            onClick={() => {
              if (todayGameId != null) onSelectGame(todayGameId);
            }}
          >
            Today
          </button>
          <button
            type="button"
            aria-label="Next game"
            disabled={orderedGames.findIndex((game) => game.id === activeGameId) >= orderedGames.length - 1}
            onClick={() => selectAdjacent(1)}
          >→</button>
        </div>
      </div>

      {loading && orderedGames.length === 0 ? (
        <div className="mlb-schedule-loading"><SportsSpinner label="Loading schedule" /></div>
      ) : orderedGames.length === 0 ? (
        <div className="mlb-schedule-empty muted">No games are available for this season.</div>
      ) : (
        <div ref={scrollerRef} className="mlb-schedule-scroller">
          <div className="mlb-schedule-track">
            {orderedGames.map((game) => {
              const isToday = gameDateKey(game) === today;
              const isActive = game.id === activeGameId;
              const away = game.awayTeam;
              const home = game.homeTeam;
              const hasScore = game.awayScore != null && game.homeScore != null;
              const result = teamGameResult(game, team.id);
              return (
                <article
                  key={game.id}
                  data-game-id={game.id}
                  className={`mlb-game-card ${isActive ? "active" : ""} ${isToday ? "today" : ""}`}
                  aria-current={isActive ? "true" : undefined}
                >
                  <button
                    type="button"
                    className="mlb-game-card-hitarea"
                    aria-label={`Open ${away.name} at ${home.name}`}
                    onClick={() => onSelectGame(game.id)}
                  />
                  <div className="mlb-game-card-topline">
                    <span>{formatGameDay(game)}</span>
                    <span className={`mlb-game-status status-${game.status}`}>
                      {isToday ? "Today" : statusLabel(game)}
                    </span>
                  </div>
                  <div className="mlb-card-team-row">
                    <span>
                      {away.logoUrl ? <img src={away.logoUrl} alt="" className="sports-logo" /> : null}
                      <TeamLink team={away} season={season} onSelectTeam={onSelectTeam} />
                    </span>
                    <strong>{hasScore ? game.awayScore : ""}</strong>
                  </div>
                  <div className="mlb-card-team-row">
                    <span>
                      {home.logoUrl ? <img src={home.logoUrl} alt="" className="sports-logo" /> : null}
                      <TeamLink team={home} season={season} onSelectTeam={onSelectTeam} />
                    </span>
                    <strong>{hasScore ? game.homeScore : ""}</strong>
                  </div>
                  <div className="mlb-game-card-context">
                    {result ? (
                      <span
                        className={`mlb-game-result ${result}`}
                        aria-label={result === "win" ? "Win" : "Loss"}
                      >
                        {result === "win" ? "W" : "L"}
                      </span>
                    ) : null}
                    <span>{gameContext(game)}</span>
                  </div>
                </article>
              );
            })}
          </div>
        </div>
      )}
    </section>
  );
}

function pitcherShortName(name: string): string {
  const parts = name.trim().split(/\\s+/);
  return parts[parts.length - 1] || name;
}

function HorizontalGameTimeline({
  detail,
  season,
  hoveredInning,
  onHoverInning,
  onScrollPlays,
  onSelectTeam,
}: {
  detail: MlbGameDetail;
  season: number | null;
  hoveredInning: number | null;
  onHoverInning: (inning: number | null) => void;
  onScrollPlays: (deltaY: number) => void;
  onSelectTeam: (teamId: number) => void;
}) {
  const inningColumnsRef = useRef<HTMLDivElement>(null);
  const inningCount = Math.max(0, ...detail.innings.map((inning) => inning.number));
  const inningByNumber = useMemo(
    () => new Map(detail.innings.map((inning) => [inning.number, inning])),
    [detail.innings],
  );
  const stints = useMemo(() => pitcherStints(detail.plays), [detail.plays]);

  useEffect(() => {
    const inningColumns = inningColumnsRef.current;
    if (!inningColumns) return;

    const handleWheel = (event: WheelEvent) => {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      const target = event.target;
      if (!(target instanceof Element) || !target.closest("button")) return;
      event.preventDefault();
      event.stopPropagation();
      onScrollPlays(event.deltaY);
    };

    inningColumns.addEventListener("wheel", handleWheel, { passive: false });
    return () => inningColumns.removeEventListener("wheel", handleWheel);
  }, [onScrollPlays]);

  if (inningCount === 0) return null;

  const innings = Array.from({ length: inningCount }, (_, index) => index + 1);
  const lane = (fieldingSide: "away" | "home", team: MlbTeam) => (
    <div className={`mlb-pitcher-lane ${fieldingSide}`}>
      <TeamLink
        team={team}
        season={season}
        onSelectTeam={onSelectTeam}
        className="mlb-pitcher-lane-label"
      >
        {teamLabel(team)} P
      </TeamLink>
      <div className="mlb-pitcher-track">
        {innings.map((inning) => <i key={inning} style={{ left: `${((inning - 1) / inningCount) * 100}%` }} />)}
        {stints.filter((stint) => stint.fieldingSide === fieldingSide).map((stint) => {
          const left = (stint.start / inningCount) * 100;
          const width = Math.max(((stint.end - stint.start) / inningCount) * 100, 1.5);
          const range = stint.startInning === stint.endInning
            ? `inning ${stint.startInning}`
            : `innings ${stint.startInning}–${stint.endInning}`;
          return (
            <span
              key={stint.key}
              className={`mlb-pitcher-stint ${stint.startingPitcher ? "starter" : "reliever"}`}
              style={{ left: `${left}%`, width: `${width}%` }}
              title={`${stint.pitcherName} · ${range}`}
              aria-label={`${stint.pitcherName}, ${range}`}
            >
              <b>{pitcherShortName(stint.pitcherName)}</b>
            </span>
          );
        })}
      </div>
      <span />
    </div>
  );

  return (
    <div className="mlb-horizontal-timeline-scroll">
      <div className="mlb-horizontal-timeline" style={{ minWidth: Math.max(590, 142 + inningCount * 48) }}>
        <div className="mlb-horizontal-linescore">
          <div className="mlb-linescore-team-labels">
            <span>Inn</span>
            <strong>
              <TeamLink team={detail.game.awayTeam} season={season} onSelectTeam={onSelectTeam} />
            </strong>
            <strong>
              <TeamLink team={detail.game.homeTeam} season={season} onSelectTeam={onSelectTeam} />
            </strong>
          </div>
          <div ref={inningColumnsRef} className="mlb-inning-columns" style={{ gridTemplateColumns: `repeat(${inningCount}, minmax(44px, 1fr))` }}>
            {innings.map((number) => {
              const inning = inningByNumber.get(number);
              const active = hoveredInning === number;
              return (
                <button
                  key={number}
                  type="button"
                  className={active ? "active" : ""}
                  aria-label={`Inning ${number}: ${inning?.awayRuns ?? 0} to ${inning?.homeRuns ?? 0}`}
                  onMouseEnter={() => onHoverInning(number)}
                  onMouseLeave={() => onHoverInning(null)}
                  onFocus={() => onHoverInning(number)}
                  onBlur={() => onHoverInning(null)}
                >
                  <span>{number}</span>
                  <strong>{inning?.awayRuns ?? "–"}</strong>
                  <strong>{inning?.homeRuns ?? "–"}</strong>
                </button>
              );
            })}
          </div>
          <div className="mlb-linescore-totals">
            <span><b>R</b><b>H</b><b>E</b></span>
            <strong><b>{detail.game.awayScore ?? 0}</b><b>{detail.awayHits ?? "–"}</b><b>{detail.awayErrors ?? "–"}</b></strong>
            <strong><b>{detail.game.homeScore ?? 0}</b><b>{detail.homeHits ?? "–"}</b><b>{detail.homeErrors ?? "–"}</b></strong>
          </div>
        </div>
        <div className="mlb-pitcher-timeline" aria-label="Pitcher usage timeline">
          {lane("home", detail.game.homeTeam)}
          {lane("away", detail.game.awayTeam)}
        </div>
      </div>
    </div>
  );
}

function GamePane({
  detail,
  loading,
  season,
  onSelectTeam,
}: Pick<Props, "detail" | "season" | "onSelectTeam"> & { loading: boolean }) {
  const [scoringOnly, setScoringOnly] = useState(true);
  const [hoveredInning, setHoveredInning] = useState<number | null>(null);
  const playFeedRef = useRef<HTMLDivElement>(null);
  const changes = useMemo(() => pitcherChanges(detail?.plays ?? []), [detail?.plays]);
  const changeByPlay = useMemo(
    () => new Map(changes.map((change) => [change.playId, change])),
    [changes],
  );
  const visiblePlays = useMemo(() => {
    if (!detail) return [];
    return detail.plays.filter((play) => {
      if (hoveredInning != null && play.inning !== hoveredInning) return false;
      return !scoringOnly || play.isScoringPlay;
    });
  }, [detail, hoveredInning, scoringOnly]);

  useEffect(() => {
    setScoringOnly(true);
    setHoveredInning(null);
  }, [detail?.game.id]);

  useEffect(() => {
    if (playFeedRef.current) playFeedRef.current.scrollTop = 0;
  }, [detail?.game.id, hoveredInning, scoringOnly]);

  if (!detail) {
    return loading ? (
      <SportsLoadingPane label="Loading game…" />
    ) : (
      <div className="empty"><h2>Game details</h2><p>Select a game from the schedule.</p></div>
    );
  }

  const game = detail.game;
  const scheduled = game.status === "scheduled" || game.status === "pre_game" || game.status === "unknown";

  return (
    <section className="mlb-game-pane">
      <header className="mlb-game-header">
        <div>
          <span className={`mlb-live-label status-${game.status}`}>{statusLabel(game)}</span>
          <span className="muted">{formatGameDay(game)} · {formatGameTime(game)}</span>
        </div>
        <div className="mlb-game-scoreboard">
          <div>
            {game.awayTeam.logoUrl ? <img src={game.awayTeam.logoUrl} alt="" /> : null}
            <TeamLink team={game.awayTeam} season={season} onSelectTeam={onSelectTeam} />
            <strong>{game.awayScore ?? "—"}</strong>
          </div>
          <span className="muted">at</span>
          <div>
            {game.homeTeam.logoUrl ? <img src={game.homeTeam.logoUrl} alt="" /> : null}
            <TeamLink team={game.homeTeam} season={season} onSelectTeam={onSelectTeam} />
            <strong>{game.homeScore ?? "—"}</strong>
          </div>
        </div>
      </header>

      {detail.innings.length > 0 ? (
        <HorizontalGameTimeline
          detail={detail}
          season={season}
          hoveredInning={hoveredInning}
          onHoverInning={setHoveredInning}
          onScrollPlays={(deltaY) => playFeedRef.current?.scrollBy({ top: deltaY })}
          onSelectTeam={onSelectTeam}
        />
      ) : null}

      {scheduled && detail.plays.length === 0 ? (
        <div className="mlb-upcoming-game">
          <span className="mlb-upcoming-diamond">◇</span>
          <h2>First pitch at {formatGameTime(game)}</h2>
          <p>Play-by-play and inning scores will appear here when the game begins.</p>
        </div>
      ) : (
        <>
          <div className="mlb-plays-toolbar">
            <div aria-live="polite">
              <strong>{hoveredInning != null ? `Inning ${hoveredInning}` : "Play-by-play"}</strong>
              <span className="muted">
                {hoveredInning != null
                  ? `Showing ${scoringOnly ? "scoring" : "all"} plays · scroll over the inning to browse`
                  : "Hover an inning above to isolate its plays"}
              </span>
            </div>
            <div className="content-tabs sports-plays-filter" role="group" aria-label="Play filter">
              <button type="button" className={`content-tab ${scoringOnly ? "active" : ""}`} onClick={() => setScoringOnly(true)}>Scoring</button>
              <button type="button" className={`content-tab ${!scoringOnly ? "active" : ""}`} onClick={() => setScoringOnly(false)}>All</button>
            </div>
          </div>
          <div ref={playFeedRef} className="sports-plays mlb-play-feed">
            {detail.plays.length === 0 ? (
              <p className="muted">No plays yet.</p>
            ) : visiblePlays.length === 0 ? (
              <p className="muted">
                {hoveredInning != null
                  ? scoringOnly
                    ? `No scoring plays for inning ${hoveredInning}. Choose All to see every play.`
                    : `No recorded plays for inning ${hoveredInning}.`
                  : "No scoring plays yet. Choose All to follow every at-bat."}
              </p>
            ) : visiblePlays.map((play) => {
              const change = changeByPlay.get(play.id);
              return (
                <div key={play.id}>
                  {change ? <div className="mlb-pitcher-change"><i /> Pitching change · {change.pitcherName}</div> : null}
                  <div className={`sports-play ${play.isScoringPlay ? "scoring" : ""}`}>
                    <div className="sports-play-meta">
                      <span>{play.half === "top" ? "Top" : "Bot"} {play.inning}</span>
                      <span>{play.event}</span>
                      {play.isScoringPlay ? <span className="sports-scoring-badge">Scoring</span> : null}
                      {play.awayScore != null && play.homeScore != null ? <span>{play.awayScore}–{play.homeScore}</span> : null}
                    </div>
                    <p>{play.description}</p>
                  </div>
                </div>
              );
            })}
          </div>
        </>
      )}
    </section>
  );
}

function PlayerName({ player }: { player: MlbRosterPlayer }) {
  return (
    <span className="mlb-roster-player-name">
      {player.jerseyNumber ? <span className="mlb-roster-number">#{player.jerseyNumber}</span> : null}
      <span>{player.name}</span>
      {player.rosterStatus === "injured" ? (
        <span className="mlb-il-badge" title={player.statusDescription || "Injured list"}>IL</span>
      ) : null}
    </span>
  );
}

function RosterPane({ roster, loading }: Pick<Props, "roster"> & { loading: boolean }) {
  const hitters = roster?.players.filter((player) => player.positionType !== "Pitcher") ?? [];
  const pitchers = roster?.players.filter((player) => player.positionType === "Pitcher") ?? [];
  const injuredCount = roster?.players.filter((player) => player.rosterStatus === "injured").length ?? 0;

  return (
    <section className="mlb-roster-pane">
      <div className="mlb-pane-title-row">
        <div>
          <h2>Player stats</h2>
        </div>
        {loading ? <SportsSpinner label="Updating roster" /> : null}
      </div>
      {roster ? (
        <p className="mlb-roster-summary">
          {roster.players.length - injuredCount} active · {injuredCount} on IL · {roster.season} season
        </p>
      ) : null}
      {!roster && loading ? <SportsLoadingPane label="Loading player stats…" /> : !roster ? (
        <div className="empty"><h2>No roster</h2><p>Roster stats are not available.</p></div>
      ) : (
        <>
          <h3 className="mlb-standing-heading">Position players</h3>
          <div className="mlb-roster-table-wrap">
            <table className="sports-linescore mlb-roster-table">
              <thead><tr><th>Player</th><th>Pos</th><th>G</th><th>AB</th><th>R</th><th>H</th><th>HR</th><th>RBI</th><th>AVG</th><th>OBP</th><th>OPS</th></tr></thead>
              <tbody>
                {hitters.map((player) => (
                  <tr key={player.playerId} className={player.rosterStatus === "injured" ? "on-il" : ""}>
                    <td><PlayerName player={player} /></td><td>{player.position || "—"}</td>
                    <td>{player.batting?.games ?? "—"}</td><td>{player.batting?.atBats ?? "—"}</td>
                    <td>{player.batting?.runs ?? "—"}</td><td>{player.batting?.hits ?? "—"}</td>
                    <td>{player.batting?.homeRuns ?? "—"}</td><td>{player.batting?.rbi ?? "—"}</td>
                    <td>{player.batting?.average || "—"}</td><td>{player.batting?.onBasePercentage || "—"}</td>
                    <td>{player.batting?.ops || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <h3 className="mlb-standing-heading mlb-roster-pitching-heading">Pitchers</h3>
          <div className="mlb-roster-table-wrap">
            <table className="sports-linescore mlb-roster-table">
              <thead><tr><th>Player</th><th>G</th><th>GS</th><th>W</th><th>L</th><th>SV</th><th>IP</th><th>ERA</th><th>WHIP</th><th>SO</th><th>BB</th></tr></thead>
              <tbody>
                {pitchers.map((player) => (
                  <tr key={player.playerId} className={player.rosterStatus === "injured" ? "on-il" : ""}>
                    <td><PlayerName player={player} /></td><td>{player.pitching?.games ?? "—"}</td>
                    <td>{player.pitching?.gamesStarted ?? "—"}</td><td>{player.pitching?.wins ?? "—"}</td>
                    <td>{player.pitching?.losses ?? "—"}</td><td>{player.pitching?.saves ?? "—"}</td>
                    <td>{player.pitching?.inningsPitched || "—"}</td><td>{player.pitching?.era || "—"}</td>
                    <td>{player.pitching?.whip || "—"}</td><td>{player.pitching?.strikeOuts ?? "—"}</td>
                    <td>{player.pitching?.walks ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  );
}

export function BaseballTeamDashboard(props: Props) {
  return (
    <main className="mlb-dashboard pane">
      {props.error ? <p className="error mlb-dashboard-error">{props.error}</p> : null}
      <ScheduleRail team={props.team} season={props.season} games={props.games} activeGameId={props.activeGameId} standings={props.standings} loading={props.scheduleLoading} onSelectGame={props.onSelectGame} onSelectTeam={props.onSelectTeam} />
      <div className="mlb-dashboard-grid">
        <GamePane detail={props.detail} loading={props.detailLoading} season={props.season} onSelectTeam={props.onSelectTeam} />
        <RosterPane roster={props.roster} loading={props.rosterLoading} />
      </div>
    </main>
  );
}
