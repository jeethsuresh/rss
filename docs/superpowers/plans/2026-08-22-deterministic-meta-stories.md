# Deterministic Meta-Stories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cluster RSS articles into meta-stories with a deterministic nearest-neighbour grouper that runs on fetch, accepts thumbs feedback, and is queryable by AI triage.

**Architecture:** Pure scoring lives in `backend/internal/cluster`. A cluster `Service` loads candidates from SQLite, writes `stories` / votes / token weights, and is called after RSS insert and from vote IPC. AI triage calls `Suggest` as a read-only tool. React Stories UI shows member and story thumbs.

**Tech Stack:** Go, SQLite migrations, existing stdin/stdout IPC, React + shared `ReaderBackend` types.

## Global Constraints

- RSS text only (`rssContent` / `summary` / title) — never crawled HTML
- Read Later never clustered
- Join threshold 0.35; 72h-old stories require 0.70
- Deterministic titles: `(N) ` + centroid member title
- Go owns SQLite/domain; React uses only `window.rss`

## File map

- Create: `backend/internal/cluster/stopwords.go`, `tokenize.go`, `score.go`, `decide.go`, `service.go` + `*_test.go`
- Create: `backend/internal/storage/sqlite/migrations/011_deterministic_stories.sql`
- Modify: domain models/repos, sqlite stories/articles, application fetch + votes, AI tools, IPC, shared contract, preload, App.tsx, CSS
- Test: cluster unit tests, sqlite vote tests, shared RPC test, renderer stories helper if needed

---

### Task 1: Tokenize, stopwords, cosine, decide (pure)

**Files:** `backend/internal/cluster/*.go`

- [x] Failing tests for stopwords, title-vs-body proper nouns, cosine, join/create/72h, `(N)` titles, learned weights
- [x] Implement until `go test ./internal/cluster/...` passes

### Task 2: Migration + story source/votes/weights

**Files:** `011_deterministic_stories.sql`, domain `Story`, sqlite stories repo

- [x] Persist `source`, token weights, article votes (with member snapshot), story votes
- [x] `RemoveMember`, list RSS since, get/adjust weights, get/set votes
- [x] `go test ./internal/storage/sqlite/...` passes

### Task 3: Cluster service (fetch + thumbs + leftover re-rank)

**Files:** `backend/internal/cluster/service.go`, application fetch/votes, `cmd/desktop/main.go`

- [x] `ClusterNew`, `Suggest`, `VoteArticle`, `VoteStory`
- [x] After RSS insert, cluster new IDs; skip AI enqueue when AI is off
- [x] Thumbs-down: snapshot, weights, remove, re-rank article, re-rank leftover; undo restores snapshot
- [x] Integration test with temp SQLite

### Task 4: AI `suggest_meta_story` tool

**Files:** `backend/internal/ai/service.go`

- [x] Read-only tool + system prompt; `source=ai` on successful create/join
- [x] `go test ./internal/ai/...`

### Task 5: IPC + UI thumbs

**Files:** shared contract, preload, ipc server, App.tsx, CSS

- [x] `stories.voteArticle` / `stories.voteStory`
- [x] Member-row and story-reader thumbs; empty copy; reload list after membership change
- [x] `bun test` + `cd backend && go test ./...`

---
