# Agent notes

## Quick start
- `bun install && bun dev`
- Backend binary: `bun run backend:build` → `apps/desktop/resources/bin/rss-backend`
- Tests: `bun test` and `cd backend && go test ./...`

## Architecture invariants
- Go owns SQLite, RSS, scheduler, domain
- React uses only `window.rss` (`ReaderBackend`)
- No localhost HTTP IPC for the desktop MVP
- Do not implement `cmd/server` until desktop is stable (see `docs/future-server.md`)

## IPC
Newline-delimited JSON over stdin/stdout. Events have `event`+`payload` and no `id`.

## Phase docs
- Spec: `docs/superpowers/specs/2026-08-20-rss-reader-phases-design.md`
- Plan: `docs/superpowers/plans/2026-08-20-rss-reader-implementation.md`
- Checklist: `TODO.md`
