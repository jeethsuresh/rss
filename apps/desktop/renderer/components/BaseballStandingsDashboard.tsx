import type {
  MlbGame,
  MlbStandingSection,
  MlbStandings,
  MlbTeam,
} from "@rss-reader/shared";
import { mlbTeamHref } from "../lib/sportsDeepLinks";
import { SportsLoadingPane, SportsSpinner } from "./SportsSpinner";

type Props = {
  season: number | null;
  standings: MlbStandings | null;
  games: MlbGame[];
  gameDate: string;
  todayDate: string;
  followedTeamIds: number[];
  standingsLoading: boolean;
  gamesLoading: boolean;
  error: string | null;
  onSelectTeam: (teamId: number) => void;
  onSelectGame: (game: MlbGame) => void;
  onPreviousDay: () => void;
  onNextDay: () => void;
  onToday: () => void;
};

function TeamLink({ team, season, onSelectTeam }: {
  team: MlbTeam;
  season: number | null;
  onSelectTeam: (teamId: number) => void;
}) {
  return (
    <a
      className="mlb-team-link"
      href={mlbTeamHref(team.id, season)}
      onClick={(event) => {
        event.preventDefault();
        onSelectTeam(team.id);
      }}
    >
      {team.abbreviation || team.shortName || team.name}
    </a>
  );
}

function StandingsTable({ section, tracked, season, onSelectTeam }: {
  section: MlbStandingSection;
  tracked: ReadonlySet<number>;
  season: number | null;
  onSelectTeam: (teamId: number) => void;
}) {
  return (
    <div className="mlb-standings-table-wrap">
      <table className="sports-linescore sports-standings-table mlb-standings-table">
        <thead><tr><th>#</th><th>Team</th><th>W</th><th>L</th><th>PCT</th><th>{section.kind === "wildcard" ? "WCGB" : "GB"}</th><th>Diff</th></tr></thead>
        <tbody>
          {section.teams.map((row) => {
            const isTracked = tracked.has(row.team.id);
            return (
              <tr key={row.team.id} className={isTracked ? "tracked-team" : ""}>
                <td>{row.rank}</td>
                <td className="sports-standings-team">
                  {row.team.logoUrl ? <img src={row.team.logoUrl} alt="" className="sports-logo" /> : null}
                  <TeamLink team={row.team} season={season} onSelectTeam={onSelectTeam} />
                  {isTracked ? <b className="mlb-you-badge">Tracked</b> : null}
                </td>
                <td>{row.wins}</td><td>{row.losses}</td><td>{row.winningPercentage || "—"}</td>
                <td>{section.kind === "wildcard" ? row.wildCardGamesBack || "—" : row.gamesBack || "—"}</td>
                <td className={row.runDifferential > 0 ? "positive" : row.runDifferential < 0 ? "negative" : ""}>
                  {row.runDifferential > 0 ? "+" : ""}{row.runDifferential}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function gameStartTime(game: MlbGame): string {
  const date = new Date(game.gameDate);
  if (Number.isNaN(date.getTime())) return "TBD";
  return date.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}

function gameLeague(game: MlbGame): "AL" | "NL" {
  return game.league === "NL" || game.homeTeam.league === "NL" ? "NL" : "AL";
}

function TeamWithLogo({ team }: {
  team: MlbTeam;
}) {
  const code = team.abbreviation || team.shortName || team.name;
  return (
    <span className="mlb-today-team">
      {team.logoUrl ? <img src={team.logoUrl} alt="" className="mlb-today-team-logo" /> : null}
      <span>{code}</span>
    </span>
  );
}

function TodayGameCard({ game, tracked, onSelectGame }: {
  game: MlbGame;
  tracked: ReadonlySet<number>;
  onSelectGame: (game: MlbGame) => void;
}) {
  const trackedGame = tracked.has(game.awayTeam.id) || tracked.has(game.homeTeam.id);
  const innings = game.inningScores ?? [];
  const hasInningScores = game.status === "final" && innings.length > 0;
  const hasScore = game.status === "live" || game.status === "final";
  const awayCode = game.awayTeam.abbreviation || game.awayTeam.shortName || game.awayTeam.name;
  const homeCode = game.homeTeam.abbreviation || game.homeTeam.shortName || game.homeTeam.name;

  return (
    <article
      className={`mlb-today-game ${trackedGame ? "tracked-game" : ""} ${hasInningScores ? "has-linescore" : ""}`}
      aria-label={`Open ${game.awayTeam.name} at ${game.homeTeam.name}, ${gameStartTime(game)}`}
      tabIndex={0}
      role="button"
      onClick={() => onSelectGame(game)}
      onKeyDown={(event) => {
        if (event.key !== "Enter" && event.key !== " ") return;
        event.preventDefault();
        onSelectGame(game);
      }}
    >
      <time dateTime={game.gameDate}>{gameStartTime(game)}</time>
      <div className="mlb-compact-score">
        <div>
          <TeamWithLogo team={game.awayTeam} />
          <strong>{hasScore ? (game.awayScore ?? "–") : "–"}</strong>
        </div>
        <div>
          <TeamWithLogo team={game.homeTeam} />
          <strong>{hasScore ? (game.homeScore ?? "–") : "–"}</strong>
        </div>
      </div>
      {hasInningScores ? (
        <div className="mlb-inning-popover" role="tooltip">
          <div className="mlb-inning-popover-title">
            <strong>{awayCode} {game.awayScore ?? "–"} · {homeCode} {game.homeScore ?? "–"}</strong>
            <span>Final linescore</span>
          </div>
          <table className="mlb-today-linescore" aria-label={`${game.awayTeam.name} at ${game.homeTeam.name} linescore`}>
            <thead>
              <tr>
                <th>Team</th>
                {innings.map((inning) => <th key={inning.number}>{inning.number}</th>)}
                <th>R</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <th>{awayCode}</th>
                {innings.map((inning) => <td key={inning.number} className={(inning.awayRuns ?? 0) > 0 ? "scored" : undefined}>{inning.awayRuns ?? "–"}</td>)}
                <td><strong>{game.awayScore ?? "–"}</strong></td>
              </tr>
              <tr>
                <th>{homeCode}</th>
                {innings.map((inning) => <td key={inning.number} className={(inning.homeRuns ?? 0) > 0 ? "scored" : undefined}>{inning.homeRuns ?? "–"}</td>)}
                <td><strong>{game.homeScore ?? "–"}</strong></td>
              </tr>
            </tbody>
          </table>
        </div>
      ) : null}
    </article>
  );
}

export function BaseballStandingsDashboard(props: Props) {
  const tracked = new Set(props.followedTeamIds);
  const standingsByLeague = {
    AL: props.standings?.sections.filter((section) => section.league === "AL") ?? [],
    NL: props.standings?.sections.filter((section) => section.league === "NL") ?? [],
  };
  const gamesByLeague = {
    AL: props.games.filter((game) => gameLeague(game) === "AL"),
    NL: props.games.filter((game) => gameLeague(game) === "NL"),
  };
  const gameDate = new Date(`${props.gameDate}T12:00:00`);
  const gameDateLabel = gameDate.toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" });
  const isToday = props.gameDate === props.todayDate;

  return (
    <main className="mlb-league-dashboard pane">
      {props.error ? <p className="error mlb-dashboard-error">{props.error}</p> : null}

      <section className="mlb-today-slate" aria-label={`${gameDateLabel} MLB games`}>
        <div className="mlb-pane-title-row">
          <div><h2>{gameDateLabel}</h2></div>
          <div className="mlb-day-controls">
            {props.gamesLoading ? <SportsSpinner label="Updating games" /> : null}
            <button type="button" aria-label="Previous day" title="Previous day" onClick={props.onPreviousDay}>←</button>
            <button type="button" className="mlb-day-today" disabled={isToday} onClick={props.onToday}>Today</button>
            <button type="button" aria-label="Next day" title="Next day" disabled={isToday} onClick={props.onNextDay}>→</button>
          </div>
        </div>
        {!props.games.length && props.gamesLoading ? <SportsLoadingPane label="Loading games…" /> : !props.games.length ? (
          <p className="muted">No MLB games are scheduled for this date.</p>
        ) : (
          <div className="mlb-today-leagues">
            {(["AL", "NL"] as const).map((league) => (
              <section className="mlb-today-league-column" key={league} aria-label={league === "AL" ? "American League games" : "National League games"}>
                <h3 className={league.toLowerCase()}>
                  <span>{league}</span> {league === "AL" ? "American League" : "National League"}
                </h3>
                <div className="mlb-today-game-list">
                  {gamesByLeague[league].length ? gamesByLeague[league].map((game) => (
                    <TodayGameCard
                      key={game.id}
                      game={game}
                      tracked={tracked}
                      onSelectGame={props.onSelectGame}
                    />
                  )) : <p className="muted mlb-no-league-games">No {league} games.</p>}
                </div>
              </section>
            ))}
          </div>
        )}
      </section>

      <section className="mlb-all-standings" aria-label={`${props.season} MLB standings`}>
        <div className="mlb-standings-status">
          {props.standingsLoading ? <SportsSpinner label="Updating standings" /> : null}
        </div>
        {!props.standings && props.standingsLoading ? <SportsLoadingPane label="Loading standings…" /> : !props.standings ? (
          <div className="empty"><h2>No standings</h2><p>League standings are not available.</p></div>
        ) : (
          <div className="mlb-league-standings-grid">
            {(["AL", "NL"] as const).map((league) => (
              <section className="mlb-league-standings-column" key={league} aria-label={`${league} standings`}>
                <h2 className={league.toLowerCase()}>
                  <span>{league}</span> {league === "AL" ? "American League" : "National League"}
                </h2>
                <div className="mlb-league-standing-sections">
                  {standingsByLeague[league].map((section) => (
                    <article className="mlb-standings-card" key={section.id}>
                      <h3>{section.kind === "wildcard" ? "Wild Card" : section.name}</h3>
                      <StandingsTable section={section} tracked={tracked} season={props.season} onSelectTeam={props.onSelectTeam} />
                    </article>
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
