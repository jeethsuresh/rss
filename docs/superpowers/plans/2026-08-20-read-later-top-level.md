# Read Later Top-Level Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote Read Later to a top-level two-pane app mode with filters, archive, URL add field, and Send to Read Later from RSS.

**Architecture:** Keep the `readlater://local` feed and `is_read_later` flag. Add `archived_at` + settings `readLaterChrome`. Split renderer into RSS shell vs `ReadLaterView`. Backend add/send kick crawl asynchronously.

**Tech Stack:** Go/SQLite backend, Electron+React renderer, shared TS contracts, Bun.

## Global Constraints

- No localhost HTTP IPC; use existing `window.rss` JSON-RPC.
- Full crawled HTML + PageFrame behavior stays.
- `readLaterChrome` toggle is intentionally thin.
- Prefer empty JSON arrays `[]` over `null` for lists.

## File map

- Create: `backend/migrations/004_read_later_archive.sql`, `backend/internal/storage/sqlite/migrations/004_read_later_archive.sql`
- Create: `apps/desktop/renderer/views/ReadLaterView.tsx`
- Modify: `backend/internal/domain/models.go`, `repositories.go`
- Modify: `backend/internal/storage/sqlite/articles.go`, `settings.go`
- Modify: `backend/internal/application/readlater.go`, `crud.go` (settings)
- Modify: `backend/internal/ipc/server.go`
- Modify: `packages/shared/src/index.ts`
- Modify: `apps/desktop/electron/preload.ts`
- Modify: `apps/desktop/renderer/App.tsx`, `views/SettingsPage.tsx`, `styles/global.css`
- Modify: `TODO.md` checklist if present

---

### Task 1: Migration + domain + list/archive storage

**Files:**
- Create: both `004_read_later_archive.sql` copies
- Modify: `backend/internal/domain/models.go`, `repositories.go`
- Modify: `backend/internal/storage/sqlite/articles.go`

- [ ] **Step 1: Add migration**

```sql
ALTER TABLE articles ADD COLUMN archived_at TEXT;
```

- [ ] **Step 2: Extend Article + ArticleQuery**

Add `ArchivedAt *time.Time \`json:"archivedAt,omitempty"\`` and query fields:
`ReadLaterOnly`, `ArchivedOnly bool`, `ExcludeArchived bool` (or filter enum handled in app layer).

- [ ] **Step 3: Scan/upsert archived_at; SetArchived(ctx, id, archived bool)**

List WHERE clauses:
- filter all: `is_read_later=1 AND archived_at IS NULL`
- unread: `… AND is_read=0 AND archived_at IS NULL`
- starred: `… AND is_starred=1 AND archived_at IS NULL`
- archived: `… AND archived_at IS NOT NULL`

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/004_read_later_archive.sql backend/internal/storage/sqlite/migrations/004_read_later_archive.sql backend/internal/domain backend/internal/storage/sqlite/articles.go
git commit -m "feat: add archived_at for Read Later articles"
```

---

### Task 2: Application + IPC for list/add/archive/send

**Files:**
- Modify: `backend/internal/application/readlater.go`
- Modify: `backend/internal/ipc/server.go`
- Modify: `backend/internal/domain/models.go` (Settings)
- Modify: `backend/internal/storage/sqlite/settings.go`

- [ ] **Step 1: Make crawl async in AddReadLater**

After UpsertMany, `go` kick CrawlOne + FetchLive + AI.Enqueue; return article immediately with pending crawl.

- [ ] **Step 2: AddFromArticle(ctx, articleID)**

Load source article; call same create path with URL+title; return Read Later article.

- [ ] **Step 3: ListReadLater(ctx, filter string)**, Archive/Unarchive

- [ ] **Step 4: Settings `ReadLaterChrome` string `tabs|brandControl`, default tabs**

- [ ] **Step 5: Wire IPC methods** `readLater.list`, `readLater.add`, `readLater.addFromArticle`, `readLater.archive`, `readLater.unarchive`

- [ ] **Step 6: Build backend + commit**

```bash
bun run backend:build
git commit -am "feat: Read Later archive/send APIs and async crawl"
```

---

### Task 3: Shared types + preload

**Files:**
- Modify: `packages/shared/src/index.ts`
- Modify: `apps/desktop/electron/preload.ts`

- [ ] **Step 1: Types** — `archivedAt`, `readLaterChrome`, ReaderBackend.readLater methods, METHODS list

- [ ] **Step 2: Preload bindings**

- [ ] **Step 3: Commit**

```bash
git commit -am "feat: expose Read Later IPC in shared + preload"
```

---

### Task 4: ReadLaterView + App mode chrome

**Files:**
- Create: `apps/desktop/renderer/views/ReadLaterView.tsx`
- Modify: `apps/desktop/renderer/App.tsx`
- Modify: `apps/desktop/renderer/views/SettingsPage.tsx`
- Modify: `apps/desktop/renderer/styles/global.css`

- [ ] **Step 1: ReadLaterView** — filters, add field, 2-pane list+reader, archive actions, reuse PageFrame/content tabs helpers or duplicate minimal reader slice.

- [ ] **Step 2: App toolbar** — `appMode` state; tabs vs brandControl from settings; remove sidebar Read Later; RSS reader **Send to Read Later** button calling `addFromArticle` then `setAppMode("readLater")`.

- [ ] **Step 3: Settings General toggle** for chrome style.

- [ ] **Step 4: CSS** for mode tabs + Read Later layout.

- [ ] **Step 5: Build desktop + commit**

```bash
bun run build
git commit -am "feat: top-level Read Later two-pane mode"
```

---

### Task 5: Verify + notes

- [ ] **Step 1:** `bun run backend:build && bun test` (or relevant) + `cd backend && go test ./...`

- [ ] **Step 2:** Update `um ai edit ai-rss-reader` and `TODO.md` if checklist items exist

- [ ] **Step 3:** Final commit if docs changed
