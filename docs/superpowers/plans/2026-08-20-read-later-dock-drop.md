# Read Later Dock Drop Implementation Plan

> **For agentic workers:** Execute task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop links on the macOS Dock or the foreground window to add them to Read Later, with toast+Undo and an invalid-URL modal.

**Architecture:** Electron main forwards Dock/`open-url` payloads to the renderer; the renderer owns window drops, validation, toast, and modal. Backend gains `readLater.remove` (hard delete) and emits `article.removed`.

**Tech Stack:** Go SQLite backend, Electron IPC, React renderer, Bun tests.

## Global Constraints

- Valid URLs: `http`/`https` only (scheme-less → `https://`).
- Success: no mode switch; toast with Undo (~7s).
- Undo: hard delete, not archive.
- Skip editable-field drops.

---

### Task 1: Backend `readLater.remove`

**Files:**
- Modify: `backend/internal/domain/repositories.go` (add `Delete` on `ArticleRepository`)
- Modify: `backend/internal/storage/sqlite/articles.go`
- Modify: `backend/internal/application/readlater.go`
- Modify: `backend/internal/ipc/server.go`
- Modify: `backend/internal/storage/sqlite/repo_test.go`
- Modify: `packages/shared/src/index.ts`

- [ ] Add `Delete(ctx, id) error` on article repo; implement `DELETE FROM articles WHERE id=?`.
- [ ] `RemoveReadLater`: get article, require `is_read_later`, delete, emit `article.removed`.
- [ ] IPC `readLater.remove` + shared types/RPC list + `article.removed` event.
- [ ] Test: add read-later article, remove, Get returns not found.
- [ ] Commit.

### Task 2: Electron forward drops

**Files:**
- Modify: `apps/desktop/electron/main.ts`
- Modify: `apps/desktop/electron/preload.ts`
- Modify: `apps/desktop/renderer/lib/backend.ts`

- [ ] Queue early `open-url` events; on ready focus window and `webContents.send("desktop:dropped-text", text)`.
- [ ] Preload: `onDroppedText(handler)` + expose on `window.desktop`.
- [ ] Commit.

### Task 3: URL helper + window/toast/modal UI

**Files:**
- Create: `apps/desktop/renderer/lib/droppedUrl.ts` (+ bun test)
- Modify: `apps/desktop/renderer/App.tsx`
- Modify: `apps/desktop/renderer/styles/global.css`
- Modify: `apps/desktop/renderer/views/ReadLaterView.tsx` (listen `article.removed`)

- [ ] `normalizeDroppedUrl(raw)` → `{ ok: true, url } | { ok: false, attempted }`.
- [ ] Window drag/drop + subscribe to `desktop.onDroppedText`.
- [ ] Toast with Undo; invalid modal; wire `readLater.remove`.
- [ ] Commit.

### Task 4: Verify

- [ ] `cd backend && go test ./...`
- [ ] `bun test` / desktop tests
- [ ] `bun run backend:build`
- [ ] Update `um` note
