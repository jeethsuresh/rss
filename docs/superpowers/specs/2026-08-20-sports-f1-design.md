# Sports — Formula 1 (OpenF1) design

Date: 2026-08-20

## Goal

Add Formula 1 under Sports, parallel to Baseball: season → race list by status → race detail with classification and significant race-control events. Data from [OpenF1](https://openf1.org/docs) (`https://api.openf1.org/v1/`). UI never sees raw OpenF1 JSON.

## UX

- Sidebar **F1**: year selector; **Races** with Completed / In Progress / Scheduled (no team follow).
- Middle: races for that year + bucket.
- Detail: meeting title, classification table (pos, driver, team, points, gap/DNF), race-control timeline with All | Significant filter.
- Live races: poll ~20s via `sports.f1.race.watch` → `sports.f1.race.updated`.

## Backend

- `internal/openf1` client: meetings + Race sessions, session_result, drivers, race_control.
- Normalize to `domain.F1*` types.
- IPC: `sports.f1.years.list`, `sports.f1.races.list`, `sports.f1.race.get|watch|unwatch`.
- Significant events: non-green flags, SafetyCar, Incident, SessionStatus, and key message keywords.

## Non-goals

- Qualifying/practice sessions as first-class lists (Race only).
- Driver/team “follow” preferences.
- Standings championship table (can add later).
