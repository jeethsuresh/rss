# Sports mode — MLB baseball (v1)

**Date:** 2026-08-20  
**Status:** Approved  

## Goal

Add a top-level **Sports** mode powered by the free public MLB Stats API (`https://statsapi.mlb.com/api/v1`), integrated natively into the desktop RSS reader (Go backend + `window.rss` IPC; no paid sports APIs).

## Chrome

- Mode tabs: **`RSS Reader | Read Later | Sports`**
- Sports uses the same **3-pane** layout as RSS / Read Later.

## Sports UI

- **Sidebar:** Followed teams + “All followed”; quick follow/unfollow; link/button to Settings → Sports for full manage.
- **Middle:** Games for selection, **grouped by season** (newest first). Past / live / future. Row: date, matchup, status, score when known.
- **Right:** Game detail — teams, status, score, linescore (innings), **all plays** chronologically by inning (top/bottom preserved), scoring plays highlighted.
- Live games: auto-update via backend poll + `sports.game.updated` (no full remount).

## Team following

- Persist MLB `teamId` list in SQLite.
- Quick toggle in Sports sidebar; full checklist in **Settings → Sports**.

## Backend

- Go package fetches MLB, normalizes to domain types (`MlbTeam`, `MlbGame`, `MlbInning`, `MlbPlay`, `MlbGameDetail`).
- UI never consumes raw MLB JSON.
- Cache teams + seasons (TTL); schedule fetched per request (light in-memory cache OK).
- IPC: `sports.teams.list`, `sports.followed.get/set`, `sports.seasons.list`, `sports.schedule.list`, `sports.game.get`, `sports.game.watch` / `sports.game.unwatch` (or watch via get + open).

## Out of scope

Box score depth, other leagues, betting, notifications, paid providers.

## Success criteria

Third tab works; follow teams; seasons/schedules; game detail with linescore + all plays; live updates without reload.
