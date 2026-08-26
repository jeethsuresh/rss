# Standalone server history (Phase 8)

This document originally described the pre-server architecture. The standalone
Go server is now implemented in `backend/cmd/server`; current operation and data
model details live in [server.md](server.md).

## What to reuse as-is

- `internal/domain` models and repository interfaces
- `internal/application` services
- `internal/rss` fetch/parse/normalize/dedupe
- `internal/scheduler`
- SQLite repository implementations initially (or a new Postgres adapter behind the same interfaces)

## What was added

| Desktop | Standalone server |
|---------------|---------------|
| `cmd/desktop` + stdin/stdout JSON-RPC | `cmd/server` + HTTP/JSON |
| Electron passes `-db` userData path | Server config / env for DB DSN |
| Preload `ReaderBackend` → local IPC | Go sync client → authenticated HTTP API |

## Implemented layout

```text
backend/cmd/server/main.go
backend/internal/httpapi/
backend/internal/serverstore/
backend/internal/serverfetch/
backend/internal/syncclient/
```

## Leak checklist (keep domain clean)

Domain and application code must **not** import:

- Electron or Node types
- IPC framing types (except at the transport edge)
- UI paths, window handles, or notification APIs
- Hard-coded SQLite SQL outside `storage/sqlite`

Authentication remains at the HTTP edge and synchronization is a Go service, not
a React concern. The desktop currently synchronizes feed membership; other
tenant state adapters are tracked in `TODO.md`.

## Postgres swap

1. Implement `FeedRepository`, `ArticleRepository`, `FolderRepository`, `SettingsRepository` for Postgres.
2. Keep FTS via Postgres full-text search or a dedicated search table.
3. Wire `cmd/server` to choose storage via config while retaining its tenant overlay semantics and fetch leases.
4. Leave `cmd/desktop` on SQLite for local-first installs.
