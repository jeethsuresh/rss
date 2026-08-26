import { useEffect, useMemo, useRef, useState } from "react";
import type {
  F1Race,
  F1RaceDetail,
  F1Session,
  F1SessionKind,
  F1Standings,
} from "@rss-reader/shared";
import {
  classificationResults,
  chronologicalRaces,
  constructorsWithDrivers,
  driverContributionPercent,
  preferredRaceMeetingKey,
} from "../lib/f1Dashboard";
import { SportsLoadingPane, SportsSpinner } from "./SportsSpinner";

type KindPill = { id: F1SessionKind; label: string };

type Props = {
  year: number | null;
  races: F1Race[];
  activeMeetingKey: number | null;
  activeSessionKey: number | null;
  activeSessionKind: F1SessionKind;
  detail: F1RaceDetail | null;
  standings: F1Standings | null;
  availableKinds: KindPill[];
  kindSessions: F1Session[];
  racesLoading: boolean;
  standingsLoading: boolean;
  detailLoading: boolean;
  error: string | null;
  onSelectRace: (race: F1Race) => void;
  onSelectKind: (kind: F1SessionKind) => void;
  onSelectSession: (sessionKey: number) => void;
};

function statusLabel(status: F1Race["status"]): string {
  switch (status) {
    case "in_progress":
      return "Live";
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

function raceDay(dateStart: string): { day: string; date: string } {
  const date = new Date(dateStart);
  if (Number.isNaN(date.getTime())) return { day: "TBD", date: dateStart || "Date TBD" };
  return {
    day: date.toLocaleDateString(undefined, { day: "2-digit" }),
    date: date.toLocaleDateString(undefined, { month: "short", weekday: "short" }),
  };
}

function sessionDate(dateStart: string): string {
  const date = new Date(dateStart);
  if (Number.isNaN(date.getTime())) return dateStart;
  return date.toLocaleString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function resultStatus(result: F1RaceDetail["results"][number]): string {
  if (result.dsq) return "DSQ";
  if (result.dns) return "DNS";
  if (result.dnf) return "DNF";
  return result.gapToLeader || "";
}

function ChampionshipPane({
  year,
  standings,
  loading,
}: Pick<Props, "year" | "standings"> & { loading: boolean }) {
  const [expandedTeams, setExpandedTeams] = useState<ReadonlySet<string>>(() => new Set());
  const constructors = useMemo(
    () => constructorsWithDrivers(standings?.constructors ?? [], standings?.drivers ?? []),
    [standings],
  );

  useEffect(() => setExpandedTeams(new Set()), [year]);

  const toggleTeam = (teamName: string) => {
    setExpandedTeams((previous) => {
      const next = new Set(previous);
      if (next.has(teamName)) next.delete(teamName);
      else next.add(teamName);
      return next;
    });
  };

  return (
    <section id="f1-standings" className="f1-championship-pane" aria-label={`${year ?? "F1"} championship standings`}>
      <div className="f1-standings-context">
        {standings?.meetingName ? (
          <span>Through {standings.meetingName}</span>
        ) : <span>{year ?? "—"} season</span>}
        {loading ? <SportsSpinner label="Updating standings" /> : null}
      </div>

      <div className="f1-championship-sections">
        <section className="f1-standing-section" aria-labelledby="f1-wcc-title">
          <div className="f1-standing-title-row">
            <div>
              <h2 id="f1-wcc-title">WCC</h2>
            </div>
            <span className="f1-expand-hint">Select a team for driver split</span>
          </div>
          {loading && !standings ? (
            <SportsLoadingPane label="Loading constructors…" />
          ) : constructors.length === 0 ? (
            <p className="muted">No constructor standings available.</p>
          ) : (
            <ol className="f1-standing-list">
              {constructors.map((team) => {
                const expanded = expandedTeams.has(team.teamName);
                return (
                  <li key={team.teamName} className={`f1-constructor ${expanded ? "expanded" : ""}`}>
                    <button
                      type="button"
                      className="f1-standing-row f1-constructor-row"
                      aria-expanded={expanded}
                      onClick={() => toggleTeam(team.teamName)}
                    >
                      <span className="f1-standing-position">{team.position}</span>
                      <span className="f1-standing-name">{team.teamName}</span>
                      <strong>{team.points}<small> pts</small></strong>
                      <span className="f1-standing-chevron" aria-hidden="true">{expanded ? "−" : "+"}</span>
                    </button>
                    {expanded ? (
                      <div className="f1-driver-split">
                        {team.drivers.length === 0 ? (
                          <p className="muted">No driver split available.</p>
                        ) : (
                          <>
                            <div className="f1-contribution-labels">
                              {team.drivers.slice(0, 2).map((driver, index) => {
                                const share = driverContributionPercent(driver.points, team.points);
                                return (
                                  <span key={driver.driverNumber} className={`driver-${index + 1}`}>
                                    <b>{driver.nameAcronym || driver.name}</b>
                                    <small>{driver.points} pts · {Math.round(share)}%</small>
                                  </span>
                                );
                              })}
                            </div>
                            <div className="f1-contribution-track" aria-label="Driver points contribution">
                              {team.drivers.slice(0, 2).map((driver, index) => (
                                <i
                                  key={driver.driverNumber}
                                  className={`driver-${index + 1}`}
                                  style={{ width: `${driverContributionPercent(driver.points, team.points)}%` }}
                                />
                              ))}
                            </div>
                            {team.drivers.length > 2 ? (
                              <div className="f1-additional-contributors">
                                {team.drivers.slice(2).map((driver) => (
                                  <span key={driver.driverNumber}>
                                    {driver.nameAcronym || driver.name} · {driver.points} pts
                                  </span>
                                ))}
                              </div>
                            ) : null}
                          </>
                        )}
                      </div>
                    ) : null}
                  </li>
                );
              })}
            </ol>
          )}
        </section>

        <section className="f1-standing-section" aria-labelledby="f1-wdc-title">
          <div className="f1-standing-title-row">
            <div>
              <h2 id="f1-wdc-title">WDC</h2>
            </div>
          </div>
          {loading && !standings ? (
            <SportsLoadingPane label="Loading drivers…" />
          ) : !(standings?.drivers.length) ? (
            <p className="muted">No driver standings available.</p>
          ) : (
            <ol className="f1-standing-list">
              {standings.drivers.map((driver) => (
                <li key={driver.driverNumber} className="f1-standing-row">
                  <span className="f1-standing-position">{driver.position}</span>
                  <span className="f1-standing-name">
                    <b>{driver.nameAcronym || driver.name}</b>
                    <small>{driver.teamName || "Independent"}</small>
                  </span>
                  <strong>{driver.points}<small> pts</small></strong>
                </li>
              ))}
            </ol>
          )}
        </section>
      </div>
    </section>
  );
}

function RaceTimeline({
  year,
  races,
  activeMeetingKey,
  loading,
  onSelectRace,
}: Pick<Props, "year" | "races" | "activeMeetingKey" | "onSelectRace"> & { loading: boolean }) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const centeredMeetingRef = useRef<number | null>(null);
  const ordered = useMemo(() => chronologicalRaces(races), [races]);

  useEffect(() => {
    const scroller = scrollerRef.current;
    const active = scroller?.querySelector<HTMLElement>(`[data-meeting-key="${activeMeetingKey}"]`);
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
        behavior: centeredMeetingRef.current == null ? "auto" : "smooth",
      });
      centeredMeetingRef.current = activeMeetingKey;
    });
    return () => cancelAnimationFrame(frame);
  }, [activeMeetingKey, ordered.length]);

  const selectAdjacent = (direction: -1 | 1) => {
    const index = ordered.findIndex((race) => race.meetingKey === activeMeetingKey);
    const next = ordered[index + direction];
    if (next) onSelectRace(next);
  };

  const selectLatest = () => {
    const key = preferredRaceMeetingKey(ordered);
    const race = ordered.find((item) => item.meetingKey === key);
    if (race) onSelectRace(race);
  };

  return (
    <section className="f1-timeline-section" aria-label={`${year ?? "F1"} race calendar`}>
      <div className="f1-timeline-controls">
        <div className="mlb-schedule-controls">
          {loading ? <SportsSpinner label="Updating calendar" /> : null}
          <button
            type="button"
            aria-label="Previous race"
            disabled={ordered.findIndex((race) => race.meetingKey === activeMeetingKey) <= 0}
            onClick={() => selectAdjacent(-1)}
          >←</button>
          <button
            type="button"
            className="mlb-today-button"
            disabled={ordered.length === 0}
            onClick={selectLatest}
          >
            Latest
          </button>
          <button
            type="button"
            aria-label="Next race"
            disabled={ordered.findIndex((race) => race.meetingKey === activeMeetingKey) >= ordered.length - 1}
            onClick={() => selectAdjacent(1)}
          >→</button>
        </div>
      </div>

      {loading && ordered.length === 0 ? (
        <div className="f1-timeline-empty"><SportsSpinner label="Loading races" /></div>
      ) : ordered.length === 0 ? (
        <div className="f1-timeline-empty muted">No race weekends are available for this season.</div>
      ) : (
        <div ref={scrollerRef} className="f1-timeline-scroller">
          <div className="f1-timeline-track">
            {ordered.map((race, index) => {
              const date = raceDay(race.dateStart);
              const active = race.meetingKey === activeMeetingKey;
              return (
                <article
                  key={race.meetingKey}
                  data-meeting-key={race.meetingKey}
                  className={`f1-race-card ${active ? "active" : ""}`}
                >
                  <button
                    type="button"
                    className="f1-race-card-hitarea"
                    aria-label={`Open ${race.name}`}
                    onClick={() => onSelectRace(race)}
                  />
                  <div className="f1-race-round">
                    <span>Round {index + 1}</span>
                    <span className={`f1-race-status status-${race.status}`}>{statusLabel(race.status)}</span>
                  </div>
                  <div className="f1-race-card-body">
                    <div className="f1-race-date"><strong>{date.day}</strong><span>{date.date}</span></div>
                    <div>
                      <h3>{race.name}</h3>
                      <p>{race.circuitShortName || race.location}</p>
                    </div>
                  </div>
                  <div className="f1-race-country">{race.countryName || race.countryCode || "Formula 1"}</div>
                </article>
              );
            })}
          </div>
        </div>
      )}
    </section>
  );
}

function SessionDetail({
  detail,
  activeSessionKey,
  activeSessionKind,
  availableKinds,
  kindSessions,
  loading,
  onSelectKind,
  onSelectSession,
}: Pick<
  Props,
  | "detail"
  | "activeSessionKey"
  | "activeSessionKind"
  | "availableKinds"
  | "kindSessions"
  | "onSelectKind"
  | "onSelectSession"
> & { loading: boolean }) {
  const [significantOnly, setSignificantOnly] = useState(true);

  useEffect(() => setSignificantOnly(true), [activeSessionKey]);

  const events = useMemo(() => {
    if (!detail) return [];
    return significantOnly ? detail.events.filter((event) => event.significant) : detail.events;
  }, [detail, significantOnly]);
  const results = useMemo(() => classificationResults(detail?.results ?? []), [detail]);

  if (!detail) {
    return loading || activeSessionKey != null ? (
      <SportsLoadingPane label="Loading session…" />
    ) : (
      <div className="empty">
        <h2>Choose a race</h2>
        <p>Select a weekend on the timeline to inspect its sessions.</p>
      </div>
    );
  }

  const session = detail.session;
  return (
    <section className="f1-session-detail">
      <header className="f1-session-header">
        <div>
          <span className="reader-kicker">
            {statusLabel(session?.status ?? detail.race.status)}
            {detail.race.countryName ? ` · ${detail.race.countryName}` : ""}
          </span>
          <h2>{detail.race.name}</h2>
          <p>
            {detail.race.circuitShortName || detail.race.location}
            {session?.dateStart ? ` · ${sessionDate(session.dateStart)}` : ""}
          </p>
        </div>
        {loading ? <SportsSpinner label="Updating session" /> : null}
      </header>

      <div
        className="f1-session-tabs"
        role="tablist"
        aria-label="Weekend sessions"
        style={{
          gridTemplateColumns: `repeat(${Math.max(1, availableKinds.length)}, minmax(72px, 1fr))`,
        }}
      >
        {availableKinds.map((kind) => (
          <button
            key={kind.id}
            type="button"
            role="tab"
            aria-selected={activeSessionKind === kind.id}
            className={activeSessionKind === kind.id ? "active" : ""}
            onClick={() => onSelectKind(kind.id)}
          >
            {kind.label}
          </button>
        ))}
      </div>

      {kindSessions.length > 1 ? (
        <div className="f1-session-variants" role="group" aria-label="Session selection">
          {kindSessions.map((item) => (
            <button
              key={item.sessionKey}
              type="button"
              className={item.sessionKey === activeSessionKey ? "active" : ""}
              onClick={() => onSelectSession(item.sessionKey)}
            >
              {item.sessionName}
            </button>
          ))}
        </div>
      ) : null}

      <div className="f1-session-workspace">
        <section className="f1-classification-pane" aria-labelledby="f1-classification-title">
          <div className="f1-detail-title-row">
            <div>
              <h3 id="f1-classification-title">Classification</h3>
            </div>
            <span>{detail.results.length} drivers</span>
          </div>
          {detail.results.length === 0 ? (
            <p className="muted">No classification yet.</p>
          ) : (
            <div className="f1-classification-scroll">
              <table className="f1-classification-table">
                <thead>
                  <tr><th>Pos</th><th>Driver</th><th>Team</th><th>Pts</th><th>Gap</th></tr>
                </thead>
                <tbody>
                  {results.map((result) => {
                    const outcome = resultStatus(result);
                    return (
                      <tr key={result.driverNumber}>
                        <td><strong>{result.position || "—"}</strong></td>
                        <td>
                          <b>{result.nameAcronym || result.name}</b>
                          {result.dnf || result.dns || result.dsq ? <small className="f1-outcome">{outcome}</small> : null}
                        </td>
                        <td>{result.teamName || "—"}</td>
                        <td>{result.points}</td>
                        <td>{outcome || (result.position === 1 ? "Winner" : "—")}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="f1-events-pane" aria-labelledby="f1-events-title">
          <div className="f1-detail-title-row">
            <div>
              <h3 id="f1-events-title">Events</h3>
            </div>
            <div className="f1-event-filter" role="group" aria-label="Event filter">
              <button type="button" className={!significantOnly ? "active" : ""} onClick={() => setSignificantOnly(false)}>All</button>
              <button type="button" className={significantOnly ? "active" : ""} onClick={() => setSignificantOnly(true)}>Key</button>
            </div>
          </div>
          {detail.events.length === 0 ? (
            <p className="muted">No race-control messages.</p>
          ) : events.length === 0 ? (
            <p className="muted">No significant events in this session.</p>
          ) : (
            <div className="f1-event-feed">
              {events.map((event) => (
                <article key={event.id} className={`f1-event ${event.significant ? "significant" : ""}`}>
                  <div className="f1-event-marker" aria-hidden="true" />
                  <div>
                    <div className="f1-event-meta">
                      <strong>{event.lapNumber != null ? `Lap ${event.lapNumber}` : event.category}</strong>
                      <span>{event.flag || event.category}</span>
                      <time>{event.date ? new Date(event.date).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" }) : ""}</time>
                    </div>
                    <p>{event.message}{event.driverName ? ` · ${event.driverName}` : ""}</p>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </section>
  );
}

export function F1Dashboard(props: Props) {
  return (
    <main className="f1-dashboard pane">
      {props.error ? <p className="error f1-dashboard-error">{props.error}</p> : null}
      <ChampionshipPane
        year={props.year}
        standings={props.standings}
        loading={props.standingsLoading}
      />
      <section className="f1-race-pane">
        <RaceTimeline
          year={props.year}
          races={props.races}
          activeMeetingKey={props.activeMeetingKey}
          loading={props.racesLoading}
          onSelectRace={props.onSelectRace}
        />
        <SessionDetail
          detail={props.detail}
          activeSessionKey={props.activeSessionKey}
          activeSessionKind={props.activeSessionKind}
          availableKinds={props.availableKinds}
          kindSessions={props.kindSessions}
          loading={props.detailLoading}
          onSelectKind={props.onSelectKind}
          onSelectSession={props.onSelectSession}
        />
      </section>
    </main>
  );
}
