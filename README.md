# RSS Reader

Local-first desktop RSS reader. **Electron** is the shell; a **Go** child process owns feeds, polling, and SQLite. Designed so the same Go application can later run as a remote server without rewriting domain logic.

## Architecture

See [docs/architecture.md](docs/architecture.md) and [docs/future-server.md](docs/future-server.md).

```text
React UI → preload API → Electron main → stdin/stdout JSON-RPC → Go → SQLite
```

## Prerequisites

- [Bun](https://bun.sh) 1.4+
- Go 1.22+
- macOS, Windows, or Linux

## Setup

```bash
bun install
```

## Development

```bash
bun dev
```

This builds the Go backend, bundles Electron main/preload, starts Vite, and launches Electron. Child processes are cleaned up on exit.

Optional seed of a sample feed:

```bash
RSS_SEED=1 bun dev
```

## Tests

```bash
bun test
cd backend && go test ./...
```

## Build backend binary

```bash
bun run backend:build
```

Writes `apps/desktop/resources/bin/rss-backend` (or `.exe` on Windows).

## Packaging

```bash
bun run package
```

Cross-compile helpers:

```bash
./scripts/build-backend-all.sh
```

Targets: macOS arm64/x64, Windows x64, Linux x64. Electron packaging uses `electron-builder` when configured; see `apps/desktop/electron-builder.yml`.

## Where is the database?

Under the Electron `userData` directory, file `rss.db` (path also available via `system.info`).

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `j` / `k` | Next / previous article |
| `o` | Open original in system browser |
| `r` / `u` | Mark read / unread |
| `s` | Star / unstar |
| `f` | Refresh all feeds |
| `/` | Focus search |

## Project layout

```text
apps/desktop/     Electron + React + Vite
backend/          Go application (cmd/desktop)
packages/shared/  Shared TypeScript API contract
docs/             Architecture, phases, build spec
TODO.md           Phase checklist
```

## Phase plan

See [docs/superpowers/specs/2026-08-20-rss-reader-phases-design.md](docs/superpowers/specs/2026-08-20-rss-reader-phases-design.md).
