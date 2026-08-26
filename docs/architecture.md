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

## Standalone server transport

```text
Desktop sync client → HTTP/JSON → Go cmd/server → shared application services → SQLite
```

Domain, RSS, scheduler, and repository interfaces stay. Only transport and storage adapters change.

The server keeps canonical fetched data global and uses tenant overlay tables for
subscriptions and personal state. See [server.md](server.md).

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
- Server passwords are salted PBKDF2 hashes; bearer tokens are stored only as SHA-256 hashes
- Server URL fetches reject private, loopback, link-local, and carrier-grade NAT destinations
- Cross-process database leases prevent duplicate feed, crawl, and sports fetches

## Storage location

SQLite lives under the Electron userData directory (platform-aware), path passed to Go on startup.
The standalone server uses a separately configured SQLite path (`RSS_SERVER_DB`).

## Key packages

```text
apps/desktop/          Electron + React + Vite
backend/cmd/desktop/   Desktop entrypoint
backend/cmd/server/    Multi-tenant HTTP server entrypoint
backend/internal/      domain, application, transports, fetchers, sync, storage
packages/shared/       Shared TypeScript API types (optional mirror of Go contract)
```
