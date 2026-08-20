# Remove Dota 2 (hard delete)

**Date:** 2026-08-20  
**Status:** Approved for implementation

## Goal

Remove Dota 2 from the product for now: no UI, IPC, backend clients, settings, or SQLite Dota state. Sports retains MLB + F1 only.

## Non-goals

- Do not delete historical Dota specs/plans under `docs/superpowers/`.
- Do not change MLB or F1 behavior.
- Do not rewrite old migrations `007`–`009`; add a new drop migration instead.

## Removals

### UI
- Delete `DotaSportsPanel.tsx`.
- Remove `dota` from `sportsRegistry` and any Sports view wiring.

### Settings
- Remove Dota provider toggle and PandaScore/STRATZ token fields from Settings → Sports.

### Shared / IPC
- Remove Dota types and `sports.dota.*` methods/events from `packages/shared`, preload, and Go IPC server.

### Backend
- Delete packages: `opendota/`, `pandascore/`, `stratz/`.
- Delete `domain/dota.go`, `application/sports_dota*.go`.
- Unwire clients from `SportsService` / `cmd/desktop`.
- Remove Dota settings fields and follow/pin repo APIs.

### Database
- Migration `010_drop_dota.sql`: drop follow/pin tables; rebuild `settings` without Dota token/provider columns; clear `sports_cache` keys matching `dota.%`.

## Docs
- Keep existing Dota design/plan markdown.
- Update `TODO.md` to note Dota removed/deferred.

## Done when
- App builds; tests pass without Dota packages.
- Sports UI shows only Baseball + F1.
- Existing user DBs migrate cleanly via `010`.
