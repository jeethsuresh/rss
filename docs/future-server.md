# Future server readiness (Phase 8)

This document describes how to add a standalone server **without rewriting** domain logic. **Do not implement the server in the current MVP.**

## What to reuse as-is

- `internal/domain` models and repository interfaces
- `internal/application` services
- `internal/rss` fetch/parse/normalize/dedupe
- `internal/scheduler`
- SQLite repository implementations initially (or a new Postgres adapter behind the same interfaces)

## What to replace

| Desktop today | Future server |
|---------------|---------------|
| `cmd/desktop` + stdin/stdout JSON-RPC | `cmd/server` + HTTP/JSON (or gRPC) |
| Electron passes `-db` userData path | Server config / env for DB DSN |
| Preload `ReaderBackend` → local IPC | `ReaderBackend` → HTTP client |

## Suggested layout later

```text
backend/cmd/server/main.go
backend/internal/transport/http/
backend/internal/storage/postgres/
```

## Leak checklist (keep domain clean)

Domain and application code must **not** import:

- Electron or Node types
- IPC framing types (except at the transport edge)
- UI paths, window handles, or notification APIs
- Hard-coded SQLite SQL outside `storage/sqlite`

## Auth / sync

Out of scope until the desktop product is stable. When added, authentication belongs at the HTTP transport layer; sync should be a separate application service, not a React concern.

## Postgres swap

1. Implement `FeedRepository`, `ArticleRepository`, `FolderRepository`, `SettingsRepository` for Postgres.
2. Keep FTS via Postgres full-text search or a dedicated search table.
3. Wire `cmd/server` to choose storage via config.
4. Leave `cmd/desktop` on SQLite for local-first installs.
