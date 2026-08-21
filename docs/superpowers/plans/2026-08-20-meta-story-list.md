# Meta-story list Implementation Plan

> **For agentic workers:** Execute inline in this session. TDD: failing test first.

**Goal:** Hide singleton stories, keep Read Later out of clusters, expand members in the Stories list, and read members as full RSS articles.

**Architecture:** SQLite List/Get/AddMember/SetMembers filter non–Read Later members and require count ≥ 2. AI skips story actions for Read Later and refuses create with < 2 eligible members. React Stories mode uses flattened list rows (story + members) with the existing article reader for members.

**Tech Stack:** Go, SQLite, React, Bun test.

## Global Constraints

- React uses only `window.rss`
- No localhost HTTP IPC
- Exhaustive `switch` on unions with `never` default
- Imports at top of file

---

### Task 1: SQLite story membership

**Files:**
- Modify: `backend/internal/storage/sqlite/stories.go`
- Test: `backend/internal/storage/sqlite/repo_test.go`

- [ ] Failing tests: list hides 1-member stories; list hides stories whose second member is Read Later; Get omits Read Later; AddMember of Read Later is a no-op
- [ ] Implement member queries with `is_read_later = 0`; skip RL in AddMember/SetMembers
- [ ] `go test ./internal/storage/sqlite -run TestStory`

### Task 2: AI clustering

**Files:**
- Modify: `backend/internal/ai/service.go`

- [ ] Skip storyAction when `article.IsReadLater`
- [ ] Filter members; do not Create unless ≥ 2 eligible IDs
- [ ] SearchCompact / search_articles omit Read Later

### Task 3: Stories list UI

**Files:**
- Create: `apps/desktop/renderer/lib/stories.ts`
- Create: `apps/desktop/renderer/lib/stories.test.ts`
- Modify: `apps/desktop/renderer/App.tsx`
- Modify: `apps/desktop/renderer/styles/global.css`

- [ ] Failing tests for `storyListRows` flatten + skip Read Later members
- [ ] Accordion in article list; story summary vs member article reader; j/k over rows
- [ ] `bun test`

### Task 4: Verify and merge

- [ ] `bun test && cd backend && go test ./...`
- [ ] Rebuild backend + desktop
