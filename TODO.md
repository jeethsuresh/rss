# RSS Reader — Project TODOs

Track phase completion and follow-ups. Check items as they land.

## Phase 0 — Contracts & skeleton

- [x] Monorepo layout
- [x] Shared API contract (methods, errors, events)
- [x] `ReaderBackend` TypeScript interface
- [x] Non-goals documented

## Phase 1 — Desktop scaffold

- [x] Bun + Vite + React + Electron
- [x] Go `cmd/desktop` child process
- [x] stdin/stdout JSON-RPC + handshake
- [x] Secure preload + CSP
- [x] `bun dev` orchestration + clean shutdown
- [x] Structured logging

## Phase 2 — Persistence

- [x] Migrations runner
- [x] feeds / articles / folders / feed_folders / settings
- [x] Repository interfaces + SQLite
- [x] Dev seed mode (`-seed` / `RSS_SEED=1`)



## Phase 3 — Vertical slice

- [x] RSS/Atom fetch + parse + normalize + dedupe
- [x] Add feed flow
- [x] Three-pane UI
- [x] Mark read / star
- [x] HTML sanitization
- [x] Backend events → UI



## Phase 4 — Scheduler & feed ops

- [x] Background scheduler + backoff
- [x] ETag / Last-Modified
- [x] Folders (API + list in sidebar)
- [x] Feed status UX (error/disabled indicators)
- [x] Optional feed discovery from site URL



## Phase 5 — Reading experience

- [x] IPagination / cursors
- [x] FTS5 search
- [x] Keyboard shortcuts
- [x] Light/dark themes
- [x] Article density setting
- [x] Settings (mark-read, interval, notifications)



## Phase 6 — Hardening

- [x] HTML sanitization (isomorphic-dompurify)
- [x] Typed errors → safe UI
- [x] IPC version handshake
- [x] Process shutdown via IPC + SIGTERM
- [x] Opt-in notifications
- [x] Fixture/unit tests (fingerprint, sqlite, shared, sanitize)



## Phase 7 — Packaging

- [x] Bundle Go binary
- [x] Cross-compile script (`scripts/build-backend-all.sh`)
- [x] electron-builder config + `bun run package`
- [x] README complete



## Phase 8 — Future server (docs only)

- [x] `cmd/server` + Postgres swap documented
- [x] Leak checklist (no Electron in domain)



## Follow-ups (post-MVP)

- [ ] Folder create/assign UI polish
- [ ] Virtualized list for 10k+ articles
- [ ] Installer smoke on Windows/Linux CI
- [ ] OPML import/export
- [ ] Per-feed notification rules polish
- [x] Top-level Read Later mode (2-pane, archive, send from RSS)
- [x] Sports mode — MLB teams, schedules, live game detail (Stats API)
- [x] Sports mode — F1 races, classification, race control (OpenF1)
- [x] Sports league standings — MLB AL/NL divisions + wild card; F1 WDC/WCC
- [ ] Archive / recently-read smart views (RSS)