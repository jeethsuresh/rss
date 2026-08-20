# OpenDota-only Dota provider toggle

**Date:** 2026-08-20  
**Status:** Approved for implementation

## Goal

Add a Settings toggle so Dota 2 can run on **OpenDota only** (no API keys) versus the existing **PandaScore + STRATZ** stack. Same UI and `sports.dota.*` IPC. Prefer recent years.

## Non-goals

- Do **not** modify `backend/internal/pandascore` or `backend/internal/stratz` packages.
- Do not delete PandaScore/STRATZ support.
- Do not require perfect coverage of every amateur league.

## Settings

- New field: `dotaProvider` = `"opendota"` | `"pandascore"` (default `"pandascore"`).
- Settings → Sports: select “Dota data source”.
- When `opendota`, hide/disable PandaScore/STRATZ token fields (or show muted “not used”).
- When `pandascore`, keep current token UX.

## Follows / pins

Provider-scoped: add `provider` column to follow/pin tables (default `pandascore` for existing rows). Reads/writes use the active provider. Switching providers does not wipe the other provider’s follows/pins.

## Backend

- Expand `backend/internal/opendota` with leagues, league matches/teams, match detail → domain types.
- New `sports_dota_opendota.go`: OpenDota implementations of Dota service methods.
- Thin dispatch at the start of existing `SportsDota*` methods in `sports_dota.go` (application layer only): if provider is `opendota`, call OD helpers; else existing PS path unchanged.
- Cache keys: `dota.od.*` (never collide with `dota.*` PS/STRATZ keys).

## OpenDota mapping

| Concept | OpenDota |
|--------|----------|
| Year events | `/leagues` filtered by year in name; prefer `premium` / `professional`; recent years first |
| Event | league id (string) |
| Series (BO) | group `/leagues/{id}/matches` by `series_id` (fallback: single match id) |
| Team names | `/leagues/{id}/teams` id→name map (list rows often omit names) |
| Team search / follow | `/teams`, `/teams/{id}/matches` |
| Game detail | `/matches/{id}` picks/bans/players/scores |

## Status IPC

`sports.dota.status` includes active provider; OpenDota mode reports configured without tokens.

## UI

- Same `DotaSportsPanel`.
- Optional muted “via OpenDota” / “via PandaScore” cue.
- Empty state for PS mode still prompts for token; OpenDota mode never requires tokens.

## Priority

Most recent years matter most: year list newest-first; league list for a year sorted with premium/pro and name quality first.
