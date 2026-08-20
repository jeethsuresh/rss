# OpenDota Dota Provider Implementation Plan

> **For agentic workers:** Execute task-by-task. Do not edit `pandascore/` or `stratz/` packages.

**Goal:** Settings toggle `dotaProvider` (`opendota` | `pandascore`) with a parallel OpenDota backend path and unchanged PandaScore/STRATZ clients.

**Tech:** Go backend, SQLite migrations, shared TS types, Settings + DotaSportsPanel.

## File map

| File | Role |
|------|------|
| `backend/migrations/009_dota_provider.sql` (+ sqlite copy) | `settings.dota_provider`; follow/pin `provider` column |
| `backend/internal/opendota/*.go` | Expand client (leagues, match detail) — existing file OK to extend |
| `backend/internal/application/sports_dota_opendota.go` | OD service methods |
| `backend/internal/application/sports_dota.go` | Dispatch only at method entry |
| `backend/internal/storage/sqlite/{settings,sports}.go` | Persist provider + scoped follows/pins |
| `packages/shared`, SettingsPage, DotaSportsPanel | Types + toggle + status |

## Tasks

### Task 1: Migration + settings field

- Add `dota_provider TEXT NOT NULL DEFAULT 'pandascore'`.
- Add `provider TEXT NOT NULL DEFAULT 'pandascore'` to follow/pin tables; rebuild PKs to include provider.
- Wire Get/Update settings + shared `Settings.dotaProvider`.

### Task 2: OpenDota client APIs

- `ListLeagues`, `LeagueMatches`, `LeagueTeams`, `GetMatchDetail` → domain `DotaEvent` / `DotaMatch` / `DotaGame`.
- Unit tests for series grouping + year filter helpers.

### Task 3: OpenDota service + dispatch

- Implement OD variants of status/events/event matches/match/game/team search/team matches.
- Dispatch from existing `SportsDota*` when `dotaProvider == opendota`.
- Follow/pin repo methods take provider string.

### Task 4: Settings UI + status

- Sports section toggle; tokens only for pandascore mode.
- Status shows provider; OD empty-state without token prompt.

### Task 5: Verify

- `go test` / build backend; smoke OD events for 2026 + one game detail.
- Commit.

## Done when

- Toggle switches live data source without changing PS/STRATZ packages.
- OpenDota mode needs no API keys and prefers recent years.
- Follows/pins do not cross-contaminate between providers.
