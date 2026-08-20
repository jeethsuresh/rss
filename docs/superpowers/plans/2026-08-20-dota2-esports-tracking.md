# Dota 2 Esports Tracking Implementation Plan

> **For agentic workers:** Execute task-by-task. Adapt to Go desktop backend + `sports_cache` SWR (do not add React Query).

**Goal:** Native Dota 2 Sports tab: year → tiered events, pin events, follow teams, series/games with lazy STRATZ.

**Architecture:** PandaScore owns events/matches; STRATZ owns game detail. Tokens via `PANDASCORE_API_TOKEN` / `STRATZ_API_TOKEN` (backend only). Reuse `sports_cache` + `getOrFetch` / `queueRefresh`. Separate domain types in `domain/dota.go`.

**Tech Stack:** Go clients, SQLite follow/pin tables, IPC `sports.dota.*`, React `DotaSportsPanel`.

## Global Constraints

- Do not fold into MLB models.
- STRATZ only on series/game open.
- Rate-limit both providers; dedupe via cache layer.
- Credentials never in renderer.

---

### Task 1: Spec in repo + domain + migration

- Copy approved external spec → `docs/superpowers/specs/2026-08-20-dota2-esports-tracking-design.md` (done).
- Add `domain/dota.go`, migration `007_dota_follow_pin.sql`, repo methods.

### Task 2: Provider clients

- `backend/internal/pandascore/` — REST, bearer token, ~1s min gap (safe under 1k/hr free), 429 backoff.
- `backend/internal/stratz/` — GraphQL, bearer token, pace ~100ms, 429 backoff.
- Unit tests with `httptest`.

### Task 3: Application + IPC + shared

- `SportsService` Dota methods + cache keys `dota.*`.
- IPC + `@rss-reader/shared` types + preload.

### Task 4: UI

- Registry `dota`.
- `DotaSportsPanel.tsx`: year, tier groups, pin/follow, event matches, series games, game detail, live watch.

### Task 5: Verify

- `go test ./...`, `bun run backend:build`, commit, um note.
