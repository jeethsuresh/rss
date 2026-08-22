# Readability Clustering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist Go then JS Readability extracts, retry crawls only for connectivity/interrupts, and cluster ungrouped articles on extract-or-RSS after that attempt finishes.

**Architecture:** Crawl classifies retryability and runs `go-readability` after a successful page fetch. Failed Go extract leaves `extract_status=js` for the renderer. `articles.setExtract` stores the JS result. Clustering body reads `readerContent` when status is `ok`. RSS insert no longer calls `ClusterNew`; `clusterOne` runs when extract/crawl is terminal. Startup backfills crawled rows with `extract_status=none`.

**Tech Stack:** Go (`github.com/mackee/go-readability`), SQLite migration 012, existing IPC, renderer `@mozilla/readability`.

## Global Constraints

- RSS text only as clustering fallback — never raw crawled/live HTML as tokens
- Read Later extracted, never clustered
- One Go pass + one JS pass per successful page; Recrawl starts a new cycle
- Crawl retries: pending/none/connectivity; not 4xx/5xx/invalid
- Join threshold remains 0.50 / stale 0.70
- React uses only `window.rss`

## File map

- Create: `backend/internal/storage/sqlite/migrations/012_readability_extract.sql`
- Create: `backend/internal/crawl/classify.go`, `extract.go` + tests
- Modify: crawl service, articles repo, domain Article, cluster `rssText`, application refresh/recrawl, ipc, shared, preload, App.tsx, readerMode.ts
- Test: classify, extract fixture, sqlite needing-crawl, cluster body, shared RPC

---

### Task 1: Schema + crawl retry classification

**Files:** migration 012, domain Article, sqlite articles, crawl/classify.go

- [ ] Failing tests: 404 not in ListNeedingCrawl; timeout/pending still listed; invalid HTML classified not retryable
- [ ] Columns `reader_content`, `extract_status`, `extract_source`, `crawl_retryable`
- [ ] `SetCrawlResult` takes retryable; `ListNeedingCrawl` filters on it

### Task 2: Go extract + delayed cluster

**Files:** crawl/extract.go, crawl/service.go, cluster/service.go, application refresh, cmd/desktop

- [ ] Fixture HTML extracts non-empty article HTML via go-readability
- [ ] Nav-only HTML → extract fail
- [ ] After crawl OK, Go extract then `OnReady` → `clusterOne` if ungrouped
- [ ] Remove `clusterSince` from RSS insert
- [ ] `clusterText` prefers reader content

### Task 3: JS fallback IPC + renderer drain + reader tab

**Files:** ipc, shared, preload, App.tsx, readerMode.ts, cmd/desktop backfill

- [ ] `articles.setExtract` / `articles.pendingExtract`
- [ ] Renderer drains `js` on startup and events
- [ ] Reader prefers `readerContent` when `ok`
- [ ] Startup backfill: crawled OK + extract none → Go then JS/cluster
