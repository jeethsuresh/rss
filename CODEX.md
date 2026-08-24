# GPT-5.6 Sol Launchpad

## Mission

Ship a polished local-first RSS reader without eroding its boundaries. Verify
claims with fresh evidence and leave the real app runnable.

## Authority

1. The current user request.
2. `AGENTS.md` — repository invariants and canonical commands.
3. The relevant approved spec and plan under `docs/superpowers/`.
4. `docs/architecture.md`, `TODO.md`, tests, then existing code patterns.

Call out conflicts instead of guessing. Never rewrite unrelated user changes.

## Architecture: do not cross these lines

```text
React renderer → window.rss → preload → Electron main
  → newline-delimited JSON over stdin/stdout → Go backend → SQLite
```

- Go owns domain rules, RSS, crawling, scheduling, clustering, AI workflows,
  repositories, migrations, and SQLite.
- React owns views and transient UI state. It talks to the backend only through
  `window.rss` (`ReaderBackend`).
- Electron main/preload own process management, the narrow IPC bridge, and OS
  APIs—not product rules or persistence.
- Events are `{ "event", "payload" }` with no request `id`. Keep untrusted
  article HTML sanitized and isolated.
- Do not add localhost HTTP IPC or implement `cmd/server` while desktop is the
  active product. See `docs/future-server.md`.

## Code map

- `apps/desktop/renderer/` — React UI, hooks, and renderer-only helpers.
- `apps/desktop/electron/` — Electron main process and secure preload bridge.
- `packages/shared/src/index.ts` — TypeScript backend contract and shared types.
- `backend/cmd/desktop/` — desktop backend entrypoint and dependency wiring.
- `backend/internal/{domain,application,ipc}/` — models, use cases, transport.
- `backend/internal/storage/sqlite/` — repositories and additive migrations.
- `backend/internal/{rss,crawl,scheduler,cluster,ai}/` — focused backend systems.
- `docs/superpowers/{specs,plans}/` — approved intent and execution history.
- `TODO.md` — completion state and current follow-ups.

## First-turn launch sequence

1. Run `git status --short --branch`; preserve every pre-existing change.
2. Read `AGENTS.md`, `TODO.md`, and only the docs relevant to the request.
3. Retrieve durable context: `um ai search "<specific topic>"`.
4. Trace the narrow path from UI contract to Go handler/service/repository.
5. Read nearby tests before proposing or changing behavior.
6. State only assumptions that materially affect the result.

For new behavior, ask focused questions, compare approaches, and present the
whole design in one approval message. After approval, write the spec and plan,
then execute continuously unless the user redirects.

## Implementation rules

- Use TDD for behavior changes: failing focused test, minimal fix, passing test.
- Keep business decisions in Go; do not mirror state machines in React.
- Change Go IPC, shared types, preload, and contract tests together.
- Make SQLite schema changes with a new forward-only migration; never edit an
  already-shipped migration.
- Prefer repository interfaces and existing application-service boundaries.
- Keep imports at module scope. TypeScript switches over unions/enums must use
  an exhaustive `never` check.
- Use the package manager for dependencies and commit lock/checksum changes.
- Avoid speculative abstractions, broad cleanup, and duplicate truth.

## Verification

Run focused checks while iterating. Before claiming completion:

```bash
bun test
cd backend && go test ./...
```

When relevant: `bun run lint`, `bun run build`, and exercise UI changes through
the real `bun dev` stack. Report exact failures; never call unrun checks passing.

```bash
bun run backend:build
```

Always finish repository work by rebuilding that Electron backend binary.

## Ship cleanly

- Review `git diff` for scope, accidental formatting, generated files, secrets.
- Commit completed changes with a concise message focused on intent.
- If this task created a dedicated feature branch, merge it normally into
  `main` when verified; never force-push.
- Append durable architecture/library/API knowledge to
  `/Users/jeeth/notes/ai-rss-reader.txt`, then run
  `um ai edit ai-rss-reader` to re-index it. Never store secrets.

## Definition of done

The requested behavior or document exists; architecture boundaries still hold;
focused and full relevant checks pass; the backend binary is rebuilt; docs and
contracts match reality; changes are committed and integrated as required; and
the handoff names what changed, what was verified, and any real remaining risk.
