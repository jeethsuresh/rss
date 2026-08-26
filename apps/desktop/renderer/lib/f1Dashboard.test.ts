import { describe, expect, test } from "bun:test";
import type { F1Race } from "@rss-reader/shared";
import {
  classificationResults,
  chronologicalRaces,
  constructorsWithDrivers,
  driverContributionPercent,
  preferredRaceMeetingKey,
} from "./f1Dashboard";

function race(meetingKey: number, dateStart: string, status: F1Race["status"]): F1Race {
  return {
    meetingKey,
    sessionKey: meetingKey * 10,
    year: 2026,
    name: `Race ${meetingKey}`,
    location: "Somewhere",
    countryName: "Canada",
    circuitShortName: "Circuit",
    dateStart,
    dateEnd: dateStart,
    status,
  };
}

describe("F1 dashboard", () => {
  test("orders races chronologically and prefers the latest completed race", () => {
    const races = [
      race(3, "2026-09-01T14:00:00Z", "scheduled"),
      race(1, "2026-07-01T14:00:00Z", "completed"),
      race(2, "2026-08-30T14:00:00Z", "scheduled"),
    ];

    expect(chronologicalRaces(races).map((item) => item.meetingKey)).toEqual([1, 2, 3]);
    expect(preferredRaceMeetingKey(races)).toBe(1);
  });

  test("uses a live race only when there are no completed races", () => {
    const liveRaces = [
      race(1, "2026-07-01T14:00:00Z", "completed"),
      race(2, "2026-08-24T14:00:00Z", "in_progress"),
    ];
    expect(preferredRaceMeetingKey(liveRaces)).toBe(1);
    expect(
      preferredRaceMeetingKey(
        [race(1, "2026-07-01T14:00:00Z", "scheduled"), race(2, "2026-07-02T14:00:00Z", "in_progress")],
      ),
    ).toBe(2);
  });

  test("places non-finishers at the bottom of a classification", () => {
    const base = {
      name: "Driver",
      points: 0,
      laps: 50,
      dns: false,
      dsq: false,
      gapToLeader: "",
    };
    const ordered = classificationResults([
      { ...base, position: 3, driverNumber: 3, dnf: true },
      { ...base, position: 2, driverNumber: 2, dnf: false },
      { ...base, position: 1, driverNumber: 1, dnf: false },
      { ...base, position: 4, driverNumber: 4, dnf: true },
    ]);

    expect(ordered.map((result) => result.driverNumber)).toEqual([1, 2, 3, 4]);
  });

  test("places positioned DNFs above DNFs without a position", () => {
    const base = {
      name: "Driver",
      points: 0,
      laps: 20,
      dnf: true,
      dns: false,
      dsq: false,
    };
    const ordered = classificationResults([
      { ...base, position: 0, driverNumber: 10 },
      { ...base, position: 15, driverNumber: 15 },
      { ...base, position: 12, driverNumber: 12 },
    ]);

    expect(ordered.map((result) => result.driverNumber)).toEqual([12, 15, 10]);
  });

  test("groups drivers under their constructor and computes bounded shares", () => {
    const grouped = constructorsWithDrivers(
      [{ position: 1, teamName: "McLaren", points: 100 }],
      [
        { position: 2, driverNumber: 4, name: "Lando Norris", teamName: "McLaren", points: 60 },
        { position: 3, driverNumber: 81, name: "Oscar Piastri", teamName: "McLaren", points: 40 },
      ],
    );

    expect(grouped[0].drivers.map((driver) => driver.driverNumber)).toEqual([4, 81]);
    expect(driverContributionPercent(40, 100)).toBe(40);
    expect(driverContributionPercent(120, 100)).toBe(100);
  });
});
