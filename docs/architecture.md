# Architecture

## Overview

```text
Electron (shell)
  ├── React renderer  — UI only; talks via preload API
  ├── Main / preload  — process management, IPC bridge, OS APIs
  └── Go backend      — domain, RSS, scheduler, SQLite
```

The Go backend is the application core. Electron is the desktop shell.

## Boundaries

| Layer | Owns | Must not |
|-------|------|----------|
| React | Views, UI state, keyboard UX | RSS fetch, SQLite, business rules |
| Electron main | Spawn Go, bridge IPC, open external URLs, app paths | Domain logic, SQL |
| Go application/domain | Feeds, articles, polling, errors | Electron types, window paths |
| SQLite | Persistence | Accessed only from Go |

## Transport today

JSON-RPC–style request/response over **stdin/stdout** between Electron main and the Go child process.

- Request: `{ "id", "method", "params" }`
- Response: `{ "id", "result" }` or `{ "id", "error": { "code", "message" } }`
- Events (Go → Electron): `{ "event", "payload" }` (no `id`)

Protocol version is negotiated via `system.handshake`.

## Transport later (not implemented)

```text
React → HTTP client → Go cmd/server → same application services → Postgres
```

Domain, RSS, scheduler, and repository interfaces stay. Only transport and storage adapters change.

## Data flow (refresh)

```text
Scheduler / manual refresh
  → HTTP fetch (ETag / Last-Modified)
  → Parse RSS/Atom
  → Normalize + dedupe
  → Persist
  → Emit articles.added / feed.updated
  → Electron forwards event → React updates
```

## Security

- `contextIsolation: true`, `nodeIntegration: false`
- Restrictive CSP
- Explicit preload API only
- Article HTML is untrusted: sanitize before render; open originals in system browser
- Never grant remote content Node/Electron privileges

## Storage location

SQLite lives under the Electron userData directory (platform-aware), path passed to Go on startup.

## Key packages

```text
apps/desktop/          Electron + React + Vite
backend/cmd/desktop/   Desktop entrypoint
backend/internal/      domain, application, rss, scheduler, storage, ipc
packages/shared/       Shared TypeScript API types (optional mirror of Go contract)
```
