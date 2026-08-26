import { useState } from "react";
import { createRoot } from "react-dom/client";
import type {
  F1Race,
  F1RaceDetail,
  F1Session,
  F1SessionKind,
  F1Standings,
} from "@rss-reader/shared";
import { F1Dashboard } from "./components/F1Dashboard";
import "./styles/global.css";

const sessionKinds: Array<{ id: F1SessionKind; label: string }> = [
  { id: "practice", label: "Practice" },
  { id: "sprint_quali", label: "Sprint Quali" },
  { id: "sprint", label: "Sprint" },
  { id: "quali", label: "Quali" },
  { id: "race", label: "Race" },
];

const sessions: F1Session[] = sessionKinds.map((kind, index) => ({
  sessionKey: 500 + index,
  sessionName: kind.id === "practice" ? "Practice 1" : kind.label,
  kind: kind.id,
  dateStart: `2026-08-2${index}T14:00:00Z`,
  dateEnd: `2026-08-2${index}T16:00:00Z`,
  status: "completed",
}));

const raceNames = ["Australian Grand Prix", "Miami Grand Prix", "Monaco Grand Prix", "Canadian Grand Prix", "British Grand Prix", "Italian Grand Prix"];
const races: F1Race[] = raceNames.map((name, index) => ({
  meetingKey: 100 + index,
  sessionKey: 504,
  year: 2026,
  name,
  location: index === 2 ? "Monte Carlo" : "Circuit",
  countryName: ["Australia", "United States", "Monaco", "Canada", "United Kingdom", "Italy"][index],
  circuitShortName: index === 2 ? "Circuit de Monaco" : "Grand Prix Circuit",
  dateStart: `2026-0${index + 3}-2${index}T14:00:00Z`,
  dateEnd: `2026-0${index + 3}-2${index}T16:00:00Z`,
  status: index < 3 ? "completed" : "scheduled",
  sessions,
}));

const driverNames = ["NOR", "PIA", "VER", "RUS", "LEC", "HAM", "ANT", "ALO", "SAI", "ALB", "GAS", "HAD", "OCO", "BEA", "TSU", "HUL", "STR", "LAW", "BOR", "COL"];
const teamNames = ["McLaren", "McLaren", "Red Bull Racing", "Mercedes", "Ferrari", "Ferrari", "Mercedes", "Aston Martin", "Williams", "Williams", "Alpine", "Racing Bulls", "Haas F1 Team", "Haas F1 Team", "Red Bull Racing", "Audi", "Aston Martin", "Racing Bulls", "Audi", "Alpine"];

const standings: F1Standings = {
  year: 2026,
  sessionKey: 504,
  meetingName: "Monaco Grand Prix",
  constructors: ["McLaren", "Mercedes", "Ferrari", "Red Bull Racing", "Williams", "Aston Martin", "Haas F1 Team", "Racing Bulls", "Audi", "Alpine"].map((teamName, index) => ({ position: index + 1, teamName, points: index === 0 ? 344 : Math.max(12, 320 - index * 31) })),
  drivers: driverNames.map((name, index) => ({
    position: index + 1,
    driverNumber: index + 1,
    name,
    nameAcronym: name,
    teamName: teamNames[index],
    points: Math.max(2, 176 - index * 8),
  })),
};

const detail: F1RaceDetail = {
  race: races[2],
  session: sessions[4],
  sessions,
  results: driverNames.map((name, index) => ({
    position: index + 1,
    driverNumber: index + 1,
    name,
    nameAcronym: name,
    teamName: teamNames[index],
    points: index < 10 ? Math.max(1, 25 - index * 2) : 0,
    laps: 78,
    dnf: index === 2,
    dns: false,
    dsq: false,
    gapToLeader: index === 0 ? "" : `+${(index * 3.417).toFixed(3)}`,
  })),
  events: Array.from({ length: 14 }, (_, index) => ({
    id: `event-${index}`,
    date: `2026-05-24T15:${String(index * 3).padStart(2, "0")}:00Z`,
    category: index % 4 === 0 ? "Flag" : "Other",
    flag: index % 4 === 0 ? "YELLOW" : "CLEAR",
    lapNumber: index * 5 + 1,
    driverName: index % 3 === 0 ? driverNames[index] : undefined,
    message: index % 4 === 0 ? "Yellow flag in sector two after a stopped car." : "Track clear and session resumed.",
    significant: index % 4 === 0,
  })),
};

function Fixture() {
  const [meetingKey, setMeetingKey] = useState(races[2].meetingKey);
  const [sessionKey, setSessionKey] = useState(sessions[4].sessionKey);
  const [kind, setKind] = useState<F1SessionKind>("race");
  return (
    <div className="layout" style={{ height: "100%" }}>
      <aside className="pane sidebar">
        <div className="sports-league-section">
          <button type="button" className="section-label sports-league-toggle open"><span>▾</span><span>Formula 1</span></button>
          <div className="section-label sports-sublabel">Year</div>
          <select className="sports-season-select" defaultValue="2026"><option>2026</option><option>2025</option></select>
          <button type="button" className="nav-item nav-item-nested sports-standings-entry active">
            <span className="sports-team-row"><span aria-hidden="true">▤</span> Standings</span>
          </button>
        </div>
      </aside>
      <F1Dashboard
        year={2026}
        races={races}
        activeMeetingKey={meetingKey}
        activeSessionKey={sessionKey}
        activeSessionKind={kind}
        detail={detail}
        standings={standings}
        availableKinds={sessionKinds.filter((item) => item.id === "practice" || item.id === "quali" || item.id === "race")}
        kindSessions={sessions.filter((item) => item.kind === kind)}
        racesLoading={false}
        standingsLoading={false}
        detailLoading={false}
        error={null}
        onSelectRace={(race) => setMeetingKey(race.meetingKey)}
        onSelectKind={setKind}
        onSelectSession={setSessionKey}
      />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Fixture />);
