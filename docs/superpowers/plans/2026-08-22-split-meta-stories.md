# Split Meta-Stories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise the deterministic join threshold to 0.50 and add a reader Split action that breaks a meta-story into connected-component sub-stories (or hidden singletons) while down-weighting tokens that overlapped across the cut.

**Architecture:** Pure graph logic lives in `backend/internal/cluster` (`SplitComponents`, `CrossComponentOverlap`). `Service.Split` loads the story, applies the graph, writes membership and token weights, and emits `story.updated`. IPC `stories.split` returns `{ storyIds }`. The Stories reader adds a Split button.

**Tech Stack:** Go cluster package, existing SQLite story/token tables, stdin/stdout IPC, React `window.rss`.

## Global Constraints

- RSS text only — never crawled HTML
- Read Later never clustered or split into new stories
- Join threshold 0.50; 72h-old stories still require 0.70
- Split uses pairwise cosine ≥ 0.50 among members of **this story only** (no 72h rule, no joining other stories)
- One component → no-op (no weight change)
- Hidden stories still require ≥2 non–Read Later members
- Go owns SQLite/domain; React uses only `window.rss`

## File map

- Modify: `backend/internal/cluster/decide.go` (`JoinThreshold`)
- Create: `backend/internal/cluster/split.go` + `split_test.go`
- Modify: `backend/internal/cluster/service.go` + `service_test.go`
- Modify: `backend/internal/application/service.go` (`Clusterer`), `feeds_stories.go`
- Modify: `backend/internal/ipc/server.go`
- Modify: `packages/shared/src/index.ts`, `packages/shared/index.test.ts`
- Modify: `apps/desktop/electron/preload.ts`, `apps/desktop/renderer/App.tsx`

---

### Task 1: Join threshold 0.50 + connected-component split (pure)

**Files:**
- Modify: `backend/internal/cluster/decide.go`
- Modify: `backend/internal/cluster/decide_test.go`
- Create: `backend/internal/cluster/split.go`
- Create: `backend/internal/cluster/split_test.go`

**Interfaces:**
- Consumes: `Member`, `Vector`, `Cosine`, `Mean`, `OverlapTokens`, `JoinThreshold`
- Produces: `func SplitComponents(members []Member, threshold float64) [][]Member`; `func CrossComponentOverlap(components [][]Member) []string`

- [ ] **Step 1: Write failing threshold tests**

Add to `decide_test.go`:

```go
func TestDecideDoesNotCreateBelowJoinThreshold(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"x": 1, "y": 1, "extra": 1}, At: now}
	other := Candidate{ID: "b", Vec: Vector{"x": 1, "z": 1}, At: now}
	got := Decide(article, []Candidate{other}, nil, now, nil)
	if got.Action != ActionNone {
		t.Fatalf("0.49-ish pair must not create, got %+v", got)
	}
}

func TestDecideCreatesAtJoinThreshold(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"x": 1, "y": 1}, At: now}
	other := Candidate{ID: "b", Vec: Vector{"x": 1, "z": 1}, At: now}
	got := Decide(article, []Candidate{other}, nil, now, nil)
	if got.Action != ActionCreate {
		t.Fatalf("cosine 0.5 must create, got %+v", got)
	}
}
```

Add `split_test.go` covering mixed cluster → two components, single component, overlap tokens across the cut only.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/cluster/ -count=1`

Expected: `TestDecideCreatesAtJoinThreshold` fails while `JoinThreshold` is still 0.35 (0.5 pair already creates today — this test may pass early). `TestDecideDoesNotCreateBelowJoinThreshold` **passes today** at 0.35 if score is ~0.41. After raising the constant, the 0.5 create test is the lock.

If `TestDecideCreatesAtJoinThreshold` already passes at 0.35, keep it. The regression lock is `JoinThreshold == 0.50` via:

```go
func TestJoinThresholdIsHalf(t *testing.T) {
	if JoinThreshold != 0.50 {
		t.Fatalf("JoinThreshold=%v", JoinThreshold)
	}
}
```

That test **must fail first**.

- [ ] **Step 3: Set JoinThreshold and implement SplitComponents**

`JoinThreshold = 0.50`. Union-find / DFS on pairwise cosine ≥ threshold. Sort members in a component by id; sort components by size desc then first id. `CrossComponentOverlap` unions `OverlapTokens` of each pair of component means.

- [ ] **Step 4: Run cluster tests**

Run: `cd backend && go test ./internal/cluster/ -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/cluster
git commit -m "feat: raise story join threshold to 0.5 and split by components"
```

---

### Task 2: Cluster service Split + tests

**Files:**
- Modify: `backend/internal/cluster/service.go`
- Modify: `backend/internal/cluster/service_test.go`

**Interfaces:**
- Consumes: `SplitComponents`, `CrossComponentOverlap`, `Stories.SetMembers`, `Stories.Create`, `Stories.AdjustTokenWeights`
- Produces: `func (s *Service) Split(ctx context.Context, storyID string) ([]string, error)`

- [ ] **Step 1: Write failing service tests**

Force a mixed 4-article membership (two Kyiv + two Apple) onto one story, call `Split`, assert two listable stories, original unlistable, cross tokens down-weighted, intra-only tokens not. Second test: two near-identical articles → no-op, same story id, no weights. Third: pair with disjoint vectors (cosine 0) → zero listable stories.

- [ ] **Step 2: Run to verify fail**

Run: `cd backend && go test ./internal/cluster/ -count=1 -run Split`  
Expected: FAIL (`Split` undefined)

- [ ] **Step 3: Implement Service.Split**

Load story, skip Read Later, tokenize with current weights, `SplitComponents(..., JoinThreshold)`. One component → return `[]string{storyID}`. Else `AdjustTokenWeights(overlap, 0, 1)`, `SetMembers(storyID, nil)`, create deterministic stories for size≥2 components, emit `story.updated`, log `split story: N → M groups`, return new ids largest-first.

- [ ] **Step 4: Tests pass**

Run: `cd backend && go test ./internal/cluster/ ./internal/storage/sqlite/ -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: split a meta-story into connected-component clusters"
```

---

### Task 3: IPC + reader Split button

**Files:**
- Modify: `backend/internal/application/service.go`, `feeds_stories.go`, `ipc/server.go`
- Modify: `packages/shared/src/index.ts`, `packages/shared/index.test.ts`
- Modify: `apps/desktop/electron/preload.ts`, `apps/desktop/renderer/App.tsx`

**Interfaces:**
- Consumes: `cluster.Service.Split`
- Produces: `stories.split(storyId) => { storyIds: string[] }`

- [ ] **Step 1: Failing shared contract test**

```ts
expect(RPC_METHODS).toContain("stories.split");
```

- [ ] **Step 2: Run `bun test packages/shared/index.test.ts`** — FAIL missing method

- [ ] **Step 3: Wire RPC + UI**

Add method to `RPC_METHODS`, `ReaderBackend.stories.split`, preload, IPC case, `Clusterer.Split`, `application.SplitStory`. Reader actions: **Split** button calling `backend.stories.split`, `loadStories()`, select `storyIds[0]` when present.

- [ ] **Step 4: `bun test` and `cd backend && go test ./...`**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: add Split control to the story reader"
```
