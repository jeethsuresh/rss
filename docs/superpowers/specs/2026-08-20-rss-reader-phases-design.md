# RSS Reader — Hybrid Build Phases Design

**Date:** 2026-08-20  
**Status:** Approved  
**Strategy:** Hybrid C — full scaffold ASAP, then hydrate capabilities behind stable contracts.

## Principle

One Go backend with two possible deployment modes (desktop IPC now; HTTP server later). Electron is the desktop shell; Go owns domain, RSS, scheduler, and SQLite.

## Phases

### Phase 0 — Contracts & repo skeleton
Lock API method names, error codes, event names, `ReaderBackend` TS interface, Go request/response shapes, monorepo layout, non-goals.

### Phase 1 — Full desktop scaffold
`bun dev` starts Vite + Electron + Go child process. Security defaults (`contextIsolation`, no `nodeIntegration`, CSP). JSON-RPC over stdin/stdout. Graceful shutdown.

### Phase 2 — Persistence foundation
SQLite migrations, feeds/articles/folders/settings tables, repository interfaces + SQLite impl, dev seed mode.

### Phase 3 — First vertical product slice
Add feed → fetch/parse/normalize/dedupe → list articles → sanitize reader → mark read/star → persist across restart. Minimal three-pane UI.

### Phase 4 — Feed operations & scheduler
Per-feed polling, backoff, ETag/Last-Modified, folders, unread counts, feed status UX, optional discovery.

### Phase 5 — Reading experience hydration
Pagination/cursors, FTS5 search, keyboard shortcuts, themes, virtualized lists, density/mark-read settings.

### Phase 6 — Hardening
XSS/sanitization audit, typed errors, IPC versioning, process/DB safety, opt-in notifications, fixture tests.

### Phase 7 — Packaging
Bundle Go binary + Electron for macOS/Windows/Linux targets; README; Definition of Done walkthrough.

### Phase 8 — Future-server readiness (docs only)
Document `cmd/server` + Postgres swap path; no server code.

## Cross-cutting

| Concern | Rule |
|---------|------|
| Reliability | Runnable after each phase; clean shutdown; migrations never destroy data |
| Extensibility | UI → `ReaderBackend`; domain unaware of Electron |
| Security | Harden Electron in Phase 1; sanitize before article HTML |
| Performance | Paginate; do not load full article DB into React |
| YAGNI | No auth, sync, remote server, Docker, brokers, Redis |

## Definition of Done (product)

Install → launch → add feed → articles appear → read → star → search → close → reopen → data intact. Go architecture makes `cmd/server` later straightforward.
