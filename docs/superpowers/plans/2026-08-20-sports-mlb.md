# Sports MLB Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Top-level Sports mode with MLB teams, schedules by season, game detail (linescore + all plays), and live polling via Go IPC.

**Architecture:** Go `internal/mlb` HTTP client normalizes Stats API → domain types; SQLite stores followed team IDs; React `SportsView` mirrors Read Later 3-pane; settings Sports section.

**Tech Stack:** Go, SQLite, Electron/React/TS, Bun.

## Global Constraints

- React uses only `window.rss`; Go owns network + SQLite.
- Public MLB API only — no API keys.
- Team identity = MLB `teamId` (number).
- Empty lists return `[]` not `null`.

## File map

- Create: `backend/internal/mlb/client.go`, `normalize.go`
- Create: `backend/internal/storage/sqlite/sports.go`
- Create: `backend/migrations/005_sports.sql` (+ embed copy)
- Create: `backend/internal/application/sports.go`
- Create: `apps/desktop/renderer/views/SportsView.tsx`
- Modify: domain, ipc, main.go, shared, preload, App.tsx, SettingsPage, CSS, TODO

---

### Task 1: Domain + migration + followed storage

- [ ] Types for MlbTeam/Game/Play/Inning/GameDetail + followed repo interface
- [ ] Migration `sports_followed_teams(team_id INTEGER PRIMARY KEY)`
- [ ] Repo GetFollowed / SetFollowed
- [ ] Commit

### Task 2: MLB client + application + IPC + live watch

- [ ] Client: Teams, Seasons, Schedule(team,season), GameLive(gamePk) → normalized detail
- [ ] Status mapping from MLB codedGameState / detailedState
- [ ] Application Sports service + WatchGame polling (~18s) emit `sports.game.updated`
- [ ] Wire ipc methods + main.go Sports field
- [ ] Commit

### Task 3: Shared + preload + SportsView + Settings + App tab

- [ ] Types + RPC_METHODS + BackendEventName
- [ ] SportsView 3-pane + Settings Sports checklist
- [ ] AppMode `"sports"` third tab
- [ ] Build/test/commit
