# RSS Reader

Local-first desktop RSS reader with an optional multi-tenant **Go** server. Electron is the desktop shell; Go owns feeds, polling, synchronization, and SQLite in both deployment modes.

The SwiftUI iOS app is a server-only client: it keeps credentials in Keychain,
streams invalidations, and delegates feeds, crawling, state, AI, and sports data
to the Go server.

**Product site / downloads:** [jeethsuresh.github.io/rss](https://jeethsuresh.github.io/rss/)

## Architecture

See [docs/architecture.md](docs/architecture.md) and [docs/future-server.md](docs/future-server.md).

```text
React UI → preload API → Electron main → stdin/stdout JSON-RPC → Go → SQLite
Hosted React UI → cookie-authenticated same-origin RPC → Go server → SQLite
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

## Standalone server

Run or build the multi-tenant Go server:

```bash
RSS_SERVER_REGISTRATION_ENABLED=true bun run server:dev
bun run server:build
```

Or build and run the deployable container:

```bash
docker compose up --build -d
curl http://127.0.0.1:8787/healthz
```

Open `http://127.0.0.1:8787` for the login-gated web app. It uses the server as
the sole authority for feeds, crawls, story grouping/AI, Read Later, and sports.

See [docs/server.md](docs/server.md) for authentication, fetch deduplication,
adaptive scheduling, AI environment variables, APIs, container hardening, and
desktop synchronization.

## iOS app

The iOS 18+ app uses Swift 6, SwiftUI, strict concurrency, bearer-token auth,
and server-sent events. Generate and open its Xcode project with:

```bash
cd apps/ios
xcodegen generate
open RSSReader.xcodeproj
```

See [apps/ios/README.md](apps/ios/README.md) for its architecture and server
requirements.

## Packaging

```bash
bun run package
```

Cross-compile Go backends:

```bash
./scripts/build-backend-all.sh
```

Stage the backend binary electron-builder expects for the current (or given) platform:

```bash
./scripts/stage-backend.sh darwin arm64   # → apps/desktop/resources/bin/rss-backend
./scripts/stage-backend.sh windows amd64  # → apps/desktop/resources/bin/rss-backend.exe
```

### GitHub Releases (CI)

Push a version tag to build and publish installers for macOS (Apple Silicon), Windows (x64), and Linux (`.deb` Ubuntu/Debian, `.rpm` Fedora/RHEL, `.pacman` Arch, plus AppImage):

```bash
git tag v0.1.0
git push origin v0.1.0
```

Workflow: `.github/workflows/release.yml` (also runnable via **Actions → Release → Run workflow**).

Targets: macOS arm64, Windows x64, Linux x64. Electron packaging uses `electron-builder`; see `apps/desktop/electron-builder.yml`.

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
apps/ios/         Swift 6 + SwiftUI server client
backend/          Go application (cmd/desktop and cmd/server)
packages/shared/  Shared TypeScript API contract
docs/             Architecture, phases, build spec
TODO.md           Phase checklist
```

## Phase plan

See [docs/superpowers/specs/2026-08-20-rss-reader-phases-design.md](docs/superpowers/specs/2026-08-20-rss-reader-phases-design.md).
