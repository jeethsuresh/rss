# Remove Dota 2 Implementation Plan

> **For agentic workers:** Execute task-by-task. Historical Dota docs stay.

**Goal:** Hard-delete Dota from UI, IPC, backend, and SQLite.

## Tasks

### Task 1: Migration `010_drop_dota.sql`
- Both `backend/migrations/` and `backend/internal/storage/sqlite/migrations/`.
- Drop `sports_dota_followed_teams`, `sports_dota_pinned_events`.
- Rebuild `settings` without `pandascore_api_token`, `stratz_api_token`, `dota_provider`.
- `DELETE FROM sports_cache WHERE key LIKE 'dota.%'`.

### Task 2: Backend delete + unwire
- Delete `opendota/`, `pandascore/`, `stratz/`, `domain/dota.go`, `sports_dota*.go`.
- Clean `SportsService`, `main.go`, settings Get/Update, `feeds_stories` patch, sports repo, domain sports interface, IPC handlers.

### Task 3: Frontend + shared
- Delete `DotaSportsPanel.tsx`; remove registry/UI/settings/preload/shared Dota surfaces.

### Task 4: TODO + verify
- Update `TODO.md`; `go test ./...`; `bun run backend:build`; commit.
